package main

import (
	"fmt"
	"os"
	"os/signal"
	"slices"

	"path/filepath"
	"syscall"

	app "github.com/eado/tzndn/app"
	enc "github.com/named-data/ndnd/std/encoding"
	"github.com/named-data/ndnd/std/log"
	"github.com/named-data/ndnd/std/ndn"
	"github.com/named-data/ndnd/std/ndn/svs_ps"

	// "github.com/named-data/ndnd/std/sync"
	ndn_sync "github.com/named-data/ndnd/std/sync"
)

var groupPrefix, _ = enc.NameFromStr("/ndn/edu/ucla/cs/omar/tz")
var multicastPrefix, _ = enc.NameFromStr("/ndn/multicast")
var files = []string{"america", "europe"}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <NAME|all> <EMAIL>\n", os.Args[0])
		os.Exit(1)
	}

	nameArg := os.Args[1]
	email := os.Args[2]

	log.Default().SetLevel(log.LevelError)

	var filesToSub ([]string)
	if nameArg == "all" {
		filesToSub = files
	} else {
		filesToSub = []string{nameArg}
	}

	fmt.Fprintln(os.Stderr, "*** TZDB client started")
	fmt.Fprintf(os.Stderr, "*** Processing files: %v\n", nameArg)
	fmt.Fprintln(os.Stderr, "*** Press Ctrl+C to exit.")

	// Create a new engine
	a, err := app.NewApp(email)
	if err != nil {
		panic("Could not log into testbed")
	}
	client := a.GetClient()

	// Create client name
	clientName, _ := enc.NameFromStr("consumer")

	// Create ALO instance
	alo, err := ndn_sync.NewSvsALO(ndn_sync.SvsAloOpts{
		Name: clientName,
		Svs: ndn_sync.SvSyncOpts{
			Client:      client,
			GroupPrefix: groupPrefix,
		},
		Snapshot: &ndn_sync.SnapshotNodeHistory{
			Client:    client,
			Threshold: 100,
		},
		MulticastPrefix: multicastPrefix,
	})
	if err != nil {
		log.Error(nil, "failed to create ALO", err)
	}

	// Set error handler
	alo.SetOnError(func(err error) {
		fmt.Fprintf(os.Stderr, "*** ALO error for %v\n", err)
	})

	client.AnnouncePrefix(ndn.Announcement{Name: alo.SyncPrefix(), Expose: true})

	// Start the ALO instance
	if err = alo.Start(); err != nil {
		log.Error(nil, "failed to start ALO for", err)
	}

	alo.SetOnPublisher(func(publisher enc.Name) {
		if slices.Contains(filesToSub, publisher.String()[1:]) {
			alo.SubscribePublisher(publisher, func(pub ndn_sync.SvsPub) {
				var content []byte

				if pub.IsSnapshot {
					snapshot, err := svs_ps.ParseHistorySnap(enc.NewWireView(pub.Content), true)
					if err != nil {
						panic(err) // we encode this, so this never happens
					}
                    content = snapshot.Entries[len(snapshot.Entries)-1].Content.Join()
				} else {
                    content = pub.Bytes()
                }

				fmt.Println("*** Updating ", publisher.String())
				filePath := filepath.Join("./tzdist", publisher.String())
				os.Mkdir("./tzdist", 0777)
				os.Create(filePath)
				os.WriteFile(filePath, content, 0777)
			})
		}
	})

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	<-sigchan
}
