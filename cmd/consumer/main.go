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
	config "github.com/eado/tzndn/config"
	ndn_sync "github.com/named-data/ndnd/std/sync"
)

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
		filesToSub = config.Files
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
			GroupPrefix: config.GroupPrefix,
		},
		Snapshot: &ndn_sync.SnapshotNodeHistory{
			Client:    client,
			Threshold: 1000,
		},
		MulticastPrefix: config.MulticastPrefix,
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

	latestBoot := make(map[string]int)

	alo.SetOnPublisher(func(publisher enc.Name) {
		if slices.Contains(filesToSub, publisher.String()[1:]) {
			alo.SubscribePublisher(publisher, func(pub ndn_sync.SvsPub) {
				var content []byte

				if pub.IsSnapshot {
					snapshot, err := svs_ps.ParseHistorySnap(enc.NewWireView(pub.Content), true)
					if err != nil {
						panic(err)
					}
					content = snapshot.Entries[len(snapshot.Entries)-1].Content.Join()
				} else {
					content = pub.Bytes()
				}

				filePath := filepath.Join(config.OutputDir, publisher.String())
				os.Mkdir(config.OutputDir, 0744)
				

				if latestBoot[publisher.String()] < int(pub.BootTime) {
					latestBoot[publisher.String()] = int(pub.BootTime)
                    os.Create(filePath)
				}

                if latestBoot[publisher.String()] == int(pub.BootTime) {
                    fmt.Println("*** Updating ", publisher.String(), " seq ", pub.SeqNum)

					f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
					if err != nil {
						return
					}
					f.Write(content)
                    f.Close()
				}
			})
		}
	})

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	<-sigchan
}
