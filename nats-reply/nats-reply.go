package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"time"
	
	"github.com/nats-io/nats.go"
)

// NOTE: Can test with demo servers.
// nats-reply -s demo.nats.io <subject> <response>
// nats-reply -s demo.nats.io:4443 <subject> <response> (TLS version)

func usage() {
	log.Printf("Usage: nats-reply [-s server] [-creds file] [-t] [-q queue] <subject> <response>\n")
	flag.PrintDefaults()
}

func showUsageAndExit(exitcode int) {
	usage()
	os.Exit(exitcode)
}

func printMsg(m *nats.Msg, i int) {
	log.Printf("[#%d] Received on [%s]: '%s'\n", i, m.Subject, string(m.Data))
}

func main() {
	var urls = flag.String("s", nats.DefaultURL, "The nats server URLs (separated by comma)")
	var userCreds = flag.String("creds", "", "User Credentials File")
	var showTime = flag.Bool("t", false, "Display timestamps")
	var queueName = flag.String("q", "NATS-RPLY-22", "Queue Group Name")
	var showHelp = flag.Bool("h", false, "Show help message")
	
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
	
	if *showHelp {
		showUsageAndExit(0)
	}
	
	args := flag.Args()
	if len(args) < 2 {
		showUsageAndExit(1)
	}
	
	// Connect Options.
	opts := []nats.Option{nats.Name("NATS Sample Responder")}
	opts = setupConnOptions(opts)
	
	// Use UserCredentials
	if *userCreds != "" {
		opts = append(opts, nats.UserCredentials(*userCreds))
	}
	
	// Connect to NATS
	nc, err := nats.Connect(*urls, opts...)
	if err != nil {
		log.Fatal(err)
	}
	
	subj, reply, i := args[0], args[1], 0
	
	nc.QueueSubscribe(subj, *queueName, func(msg *nats.Msg) {
		i++
		printMsg(msg, i)
		msg.Respond([]byte(reply))
	})
	nc.Flush()
	
	if err := nc.LastError(); err != nil {
		log.Fatal(err)
	}
	
	log.Printf("Listening on [%s]", subj)
	if *showTime {
		log.SetFlags(log.LstdFlags)
	}
	
	// Setup the interrupt handler to drain so we don't miss
	// requests when scaling down.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	log.Println()
	log.Printf("Draining...")
	nc.Drain()
	log.Fatalf("Exiting")
}

func setupConnOptions(opts []nats.Option) []nats.Option {
	totalWait := 10 * time.Minute
	reconnectDelay := time.Second
	
	opts = append(opts, nats.ReconnectWait(reconnectDelay))
	opts = append(opts, nats.MaxReconnects(int(totalWait/reconnectDelay)))
	opts = append(opts, nats.DisconnectHandler(func(nc *nats.Conn) {
		log.Printf("Disconnected: will attempt reconnects for %.0fm", totalWait.Minutes())
	}))
	opts = append(opts, nats.ReconnectHandler(func(nc *nats.Conn) {
		log.Printf("Reconnected [%s]", nc.ConnectedUrl())
	}))
	opts = append(opts, nats.ClosedHandler(func(nc *nats.Conn) {
		log.Fatalf("Exiting: %v", nc.LastError())
	}))
	return opts
}