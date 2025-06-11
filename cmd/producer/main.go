package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	_ "unsafe"

	app "github.com/eado/tzndn/app"
	enc "github.com/named-data/ndnd/std/encoding"
	"github.com/named-data/ndnd/std/log"
	"github.com/named-data/ndnd/std/ndn"
	"github.com/named-data/ndnd/std/object"
	ndn_sync "github.com/named-data/ndnd/std/sync"
)

var multicastPrefix, _ = enc.NameFromStr("/ndn/multicast")
var tzdbPrefix, _ = enc.NameFromStr("tz")
var email string

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <NAME|all> <EMAIL>\n", os.Args[0])
		os.Exit(1)
	}

	nameArg := os.Args[1]
	email = os.Args[2]

	log.Default().SetLevel(log.LevelError)

	// Determine which files to process
	var err (error)
	var filesToProcess []string
	if nameArg == "all" {
		filesToProcess, err = getFilesInTzDir()
		if err != nil {
			log.Fatal(nil, "Unable to read tz directory", "err", err)
			return
		}
	} else {
		filesToProcess = []string{nameArg}
	}

    fmt.Fprintln(os.Stderr, "*** TZDB publisher started")
	fmt.Fprintf(os.Stderr, "*** Processing files: %v\n", filesToProcess)
	fmt.Fprintln(os.Stderr, "*** Press Ctrl+C to exit.")

	// Process each file
	for _, fileName := range filesToProcess {
		err := processFile(fileName)
		if err != nil {
			log.Error(nil, "Failed to process file", "file", fileName, "err", err)
			continue
		}
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	<-sigchan

}

func getFilesInTzDir() ([]string, error) {
	files, err := os.ReadDir("./tz")
	if err != nil {
		return nil, err
	}

	var result []string
	for _, file := range files {
		if !file.IsDir() {
			result = append(result, file.Name())
		}
	}
	return result, nil
}

//go:linkname onInterest github.com/named-data/ndnd/std/object.(*Client).onInterest
func onInterest(*object.Client, ndn.InterestHandlerArgs)

func processFile(fileName string) error {
	// Create a new engine
	a, err := app.NewApp(email)
	if err != nil {
		return err
	}
	client := a.GetClient()

	// Read file data
	filePath := filepath.Join("./tz", fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %v", filePath, err)
	}

	// Create client name
	clientName, _ := enc.NameFromStr(fmt.Sprintf("%s", fileName))

	tzdbName := a.GetTestbedKey().KeyName().Prefix(-2).Append(tzdbPrefix...)

	// Create ALO instance
	alo, err := ndn_sync.NewSvsALO(ndn_sync.SvsAloOpts{
		Name: clientName,
		Svs: ndn_sync.SvSyncOpts{
			Client:      client,
			GroupPrefix: tzdbName,
		},
		Snapshot: &ndn_sync.SnapshotNodeHistory{
			Client:    client,
			Threshold: 100,
		},
		MulticastPrefix: multicastPrefix,
	})
	if err != nil {
		return fmt.Errorf("failed to create ALO for %s: %v", fileName, err)
	}

	// Set error handler
	alo.SetOnError(func(err error) {
		fmt.Fprintf(os.Stderr, "*** ALO error for %s: %v\n", fileName, err)
	})

	for _, route := range []enc.Name{
		alo.SyncPrefix(),
		alo.DataPrefix(),
	} {
		client.AnnouncePrefix(ndn.Announcement{Name: route, Expose: true})
	}

	client.Engine().AttachHandler(tzdbName, func(args ndn.InterestHandlerArgs) {
		onInterest(client.(*object.Client), args)
	})

	a.ExecWithConnectivity(func() {
		a.NotifyRepo(client, alo.SyncPrefix(), alo.DataPrefix())
	})

	// Start the ALO instance
	if err = alo.Start(); err != nil {
		return fmt.Errorf("failed to start ALO for %s: %v", fileName, err)
	}

	_, _, err = alo.Publish(enc.Wire{[]byte(data)})
	if err != nil {
		return fmt.Errorf("failed to publish for %s: %v", fileName, err)
	}

	fmt.Printf("*** Published: %s\n", fileName)
	return nil
}
