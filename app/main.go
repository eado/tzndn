// Repurposed Ownly code: https://github.com/pulsejet/ownly/blob/main/ndn/app

package app

import (
	_ "embed"
	"fmt"
	"os"
	"time"

	spec_repo "github.com/named-data/ndnd/repo/tlv"
	enc "github.com/named-data/ndnd/std/encoding"
	"github.com/named-data/ndnd/std/engine"
	"github.com/named-data/ndnd/std/engine/face"
	"github.com/named-data/ndnd/std/log"
	"github.com/named-data/ndnd/std/ndn"
	spec "github.com/named-data/ndnd/std/ndn/spec_2022"
	"github.com/named-data/ndnd/std/object"
	"github.com/named-data/ndnd/std/object/storage"
	"github.com/named-data/ndnd/std/security"
	"github.com/named-data/ndnd/std/security/keychain"
	"github.com/named-data/ndnd/std/security/ndncert"
	spec_ndncert "github.com/named-data/ndnd/std/security/ndncert/tlv"
	"github.com/named-data/ndnd/std/security/trust_schema"
	"github.com/named-data/ndnd/std/types/optional"
	"github.com/named-data/ndnd/std/utils"
    config "github.com/eado/tzndn/config"
)

//go:embed schema.tlv
var SchemaBytes []byte

//go:embed testbed.root.cert
var testbedRootCert []byte
var testbedRootName, _ = enc.NameFromStr("/ndn/KEY/%27%C4%B2%2A%9F%7B%81%27/ndn/v=1651246789556")
var testbedPrefix = enc.Name{enc.NewGenericComponent("ndn")}

func getTrustConfig(keychain ndn.KeyChain) (trust *security.TrustConfig, err error) {
	schema, err := trust_schema.NewLvsSchema(SchemaBytes)
	if err != nil {
		return
	}

	trust, err = security.NewTrustConfig(keychain, schema, []enc.Name{testbedRootName})
	if err != nil {
		return
	}
	trust.UseDataNameFwHint = true

	return
}

type App struct {
	face     ndn.Face
	engine   ndn.Engine
	store    ndn.Store
	keychain ndn.KeyChain
	client   ndn.Client
	trust    *security.TrustConfig
}

func NewApp(email string) (*App, error) {
	store := storage.NewMemoryStore()

	kc, err := keychain.NewKeyChainDir("./keychain", store)
	if err != nil {
		panic(err)
	}

	a := &App{
		store:    store,
		keychain: kc,
	}

	err = a.initialize(email)
	return a, err
}

func (a *App) initialize(email string) error {
	var err error

	// Insert trust anchor
	if err = a.keychain.InsertCert(testbedRootCert); err != nil {
		return err
	}

	// Testbed trust config
	a.trust, err = getTrustConfig(a.keychain)
	if err != nil {
		return err
	}

	err = a.ConnectTestbed()
	if err != nil {
		return err
	}

	if a.GetTestbedKey() == nil {
		err = a.NdncertEmail(email, func(status string) string {
			fmt.Print("Please enter the NDNCERT pin provided in your email: ")
			var input string
			fmt.Scanln(&input)
			return input
		})
		if err != nil {
			panic(err)
		}
	}

	signer := a.GetTestbedKey()
	if signer == nil {
		return fmt.Errorf("Could not get testbed cert")
	}

	a.client = object.NewClient(a.engine, a.store, a.trust)

	a.SetCmdKey(signer)

	return nil
}

func (a *App) GetClient() ndn.Client {
	return a.client
}

func (a *App) GetTestbedKey() ndn.Signer {
	for _, id := range a.keychain.Identities() {
		if !testbedPrefix.IsPrefix(id.Name()) {
			continue
		}

		for _, key := range id.Keys() {
			for _, certName := range key.UniqueCerts() {
				certWire, _ := a.store.Get(certName.Prefix(-1), true)
				if certWire == nil {
					log.Error(nil, "Failed to find certificate", "name", certName)
					continue
				}

				// Check if the certificate is a testbed certificate
				if certName.At(-2).String() != "NDNCERT" {
					continue
				}

				// Verify the certificate chain
				certData, err := a.verifyTestbedCert(enc.Wire{certWire}, false)
				if err != nil {
					log.Error(nil, "Failed to validate certificate", "err", err)
					continue
				}

				// Certificate is usable
				log.Info(nil, "Found valid testbed cert", "name", certData.Name())

				return key.Signer()
			}
		}
	}

	return nil
}

func (a *App) SetCmdKey(key ndn.Signer) {
	a.engine.SetCmdSec(key, func(n enc.Name, w enc.Wire, s ndn.Signature) bool {
		return true
	})
}

func (a *App) ConnectTestbed() error {
	// If we already have a face, it should automatically switch
	if a.face != nil {
		return nil
	}

	endpoint := config.Endpoint 

	face := face.NewStreamFace("udp", endpoint, false)

	a.face = face
	a.engine = engine.NewBasicEngine(a.face)
	err := a.engine.Start()
	if err != nil {
		return err
	}

	return nil
}

func (a *App) WaitForConnectivity(timeout time.Duration) error {
	if a.face != nil && a.face.IsRunning() {
		return nil
	}
	if err := a.ConnectTestbed(); err != nil {
		return err
	}
	done := make(chan struct{})
	cancel := a.face.OnUp(func() { close(done) })
	defer cancel()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for NDN connectivity")
	}
}

func (a *App) ExecWithConnectivity(callback func()) {
	if a.face.IsRunning() {
		go callback()
	} else {
		var cancel func()
		cancel = a.face.OnUp(func() {
			go callback()
			cancel()
		})
	}
}

func (a *App) verifyTestbedCert(certWire enc.Wire, fetch bool) (ndn.Data, error) {
	certData, certSigCov, err := spec.Spec{}.ReadData(enc.NewWireView(certWire))
	if err != nil {
		return nil, err
	}

	ch := make(chan error, 1)
	a.trust.Validate(security.TrustConfigValidateArgs{
		Data:              certData,
		DataSigCov:        certSigCov,
		UseDataNameFwHint: optional.Some(false), // directly available
		Fetch: func(name enc.Name, cfg *ndn.InterestConfig, callback ndn.ExpressCallbackFunc) {
			if !fetch {
				cfg.Lifetime.Set(1 * time.Millisecond) // no block
			}

			object.ExpressR(a.engine, ndn.ExpressRArgs{
				Name:     name,
				Config:   cfg,
				Retries:  utils.If(fetch, 3, 0),
				TryStore: a.store,
				Callback: callback,
			})
		}, Callback: func(valid bool, err error) {
			if err != nil {
				ch <- err
			} else if !valid {
				ch <- fmt.Errorf("certificate is not valid")
			} else {
				ch <- nil
			}
		},
	})
	return certData, <-ch
}

func (a *App) NdncertEmail(email string, CodeCb func(status string) string) (err error) {
	// Connect to the testbed
	if err := a.WaitForConnectivity(time.Second * 5); err != nil {
		return err
	}

	// Create NDNCERT client
	certClient, err := ndncert.NewClient(a.engine, testbedRootCert)
	if err != nil {
		return err
	}

	// Request a certificate from NDNCERT
	certRes, err := certClient.RequestCert(ndncert.RequestCertArgs{
		Challenge: &ndncert.ChallengeEmail{
			Email:        email,
			CodeCallback: CodeCb,
		},
		OnProfile: func(profile *spec_ndncert.CaProfile) error {
			fmt.Fprintf(os.Stderr, "NDNCERT CA: %s\n", profile.CaInfo)
			return nil
		},
		OnProbeParam: func(key string) ([]byte, error) {
			switch key {
			case ndncert.KwEmail:
				return []byte(email), nil
			default:
				return nil, fmt.Errorf("unknown probe key: %s", key)
			}
		},
		OnChooseKey: func(suggestions []enc.Name) int {
			return 0 // choose the first key
		},
		OnKeyChosen: func(keyName enc.Name) error {
			fmt.Fprintf(os.Stderr, "Certifying key: %s\n", keyName)
			return nil
		},
	})
	if err != nil {
		return err
	}

	// Verfiy the received certificate and fetch the chain
	_, err = a.verifyTestbedCert(certRes.CertWire, true)
	if err != nil {
		return fmt.Errorf("failed to verify issued certificate: %w", err)
	}

	// Store the certificate and the signer key
	if err = a.keychain.InsertKey(certRes.Signer); err != nil {
		return err
	}
	if err = a.keychain.InsertCert(certRes.CertWire.Join()); err != nil {
		return err
	}

	return nil
}

func (a *App) NotifyRepo(client ndn.Client, group enc.Name, dataPrefix enc.Name) {
	// Wait for 1s so that routes get registered
	time.Sleep(time.Second)

	// Notify repo to join SVS group
	repoCmd := spec_repo.RepoCmd{
		SyncJoin: &spec_repo.SyncJoin{
			Protocol: &spec.NameContainer{Name: spec_repo.SyncProtocolSvsV3},
            Group:    &spec.NameContainer{Name: group[2:len(group)-1]},
			HistorySnapshot: &spec_repo.HistorySnapshotConfig{
				Threshold: 100,
			},
            MulticastPrefix: &spec.NameContainer{Name: config.MulticastPrefix},
		},
	}
	client.ExpressCommand(
		config.RepoName,
		dataPrefix.Append(enc.NewKeywordComponent("repo-cmd")),
		repoCmd.Encode(),
		func(w enc.Wire, err error) {
			if err != nil {
				log.Warn(nil, "Repo sync join command failed", "group", group, "err", err)
			} else {
				log.Info(nil, "Repo joined SVS group", "group", group)
			}
		})
}
