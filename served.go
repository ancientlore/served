// served is a simple web server for static files and blogs ala golang.org.
package main

import (
	"bitbucket.org/kardianos/service"
	"flag"
	"fmt"
	"github.com/ancientlore/served/webserver"
	"log"
	"net/http"
	"os"
	"runtime/pprof"
)

var (
	cpuprofile string
	memprofile string
	wsServed   service.Service
	svcRun     bool
	conf       webserver.Config
	cfgFile    string
)

func init() {
	const (
		name        = "served"
		displayName = "Simple Web Server"
		desc        = "served is a simple web server written in Go."
		CONFIG_FILE = "served.config"
		VERSION     = "0.4"
	)

	var help bool
	var ver bool
	var svcInstall bool
	var svcRemove bool
	var svcStart bool
	var svcStop bool
	var doLog bool
	var reload bool

	defaultCfgFile := os.Getenv("SERVED_CONFIG")
	if defaultCfgFile == "" {
		defaultCfgFile = webserver.LocateConfigFile(CONFIG_FILE)
	}

	flag.StringVar(&cfgFile, "config", defaultCfgFile, "Use to override the configuration file")
	flag.BoolVar(&help, "help", false, "Show command help")
	flag.BoolVar(&ver, "version", false, "Show version")
	flag.StringVar(&cpuprofile, "cpuprofile", "", "Write CPU profile to file")
	flag.StringVar(&memprofile, "memprofile", "", "Write memory profile to file")
	flag.BoolVar(&svcInstall, "install", false, "Install served as a service")
	flag.BoolVar(&svcRemove, "remove", false, "Remove served service")
	flag.BoolVar(&svcRun, "run", false, "Run served standalone (not as a service)")
	flag.BoolVar(&svcStart, "start", false, "Start served service")
	flag.BoolVar(&svcStop, "stop", false, "Stop served service")
	flag.BoolVar(&doLog, "log", false, "Log requests")
	flag.BoolVar(&reload, "reload", false, "reload blog content on each page load")

	flag.Parse()

	if help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if ver {
		fmt.Printf("served %s\n", VERSION)
		os.Exit(0)
	}

	// read config
	conf = webserver.ReadConfig(cfgFile)
	conf.Reload = reload
	conf.Log = doLog

	var err error
	wsServed, err = service.NewService(name, displayName, desc)
	if err != nil {
		log.Fatal(err)
	}

	if svcInstall == true && svcRemove == true {
		log.Fatalln("Options -install and -remove cannot be used together.")
	} else if svcInstall == true {
		err = wsServed.Install()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Service \"%s\" installed.\n", displayName)
		os.Exit(0)
	} else if svcRemove == true {
		err = wsServed.Remove()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Service \"%s\" removed.\n", displayName)
		os.Exit(0)
	} else if svcStart == true {
		err = wsServed.Start()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Service \"%s\" started.\n", displayName)
		os.Exit(0)
	} else if svcStop == true {
		err = wsServed.Stop()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Service \"%s\" stopped.\n", displayName)
		os.Exit(0)
	}
}

func main() {
	var err error

	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if svcRun == true {
		startWork()
		sigChan := getSignalChan()
		for {
			select {
			case event := <-sigChan:
				log.Print(event)
				switch event {
				case os.Interrupt, os.Kill: //SIGINT, SIGKILL
					return
				}
			}
		}
		stopWork()
	} else {
		err = wsServed.Run(func() error {
			// start
			go startWork()
			wsServed.Info(fmt.Sprintf("Started served using config file \"%s\"", cfgFile))
			log.Printf("Started served using config file \"%s\"\n", cfgFile)
			return nil
		}, func() error {
			// stop
			stopWork()
			wsServed.Info("Stopped served")
			log.Println("Stopped served")
			return nil
		})
		if err != nil {
			wsServed.Error(err.Error())
			log.Println(err)
		}
	}
}

func startWork() {
	hm, err := webserver.CreateServers(conf)
	if err != nil {
		log.Fatal(err)
	}
	for k, v := range hm {
		go func(addr string, h http.Handler) {
			if err := http.ListenAndServe(addr, h); err != nil {
				log.Fatal("ListenAndServe: ", err)
			}
		}(k, v)
		log.Printf("Server %s added", k)
	}
}

func stopWork() {
	// write memory profile if configured
	if memprofile != "" {
		f, err := os.Create(memprofile)
		if err != nil {
			log.Print(err)
		} else {
			pprof.WriteHeapProfile(f)
			f.Close()
		}
	}
}
