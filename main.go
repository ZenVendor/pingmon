package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	probing "github.com/prometheus-community/pro-bing"
	"gopkg.in/yaml.v2"
)

const (
	MODE_STANDARD = iota
	MODE_OUTAGE
)

type Config struct {
	ConfigFile  string
	Env         string `yaml:"env"`
	DBFile      string `yaml:"dbfile"`
	StdSite     string `yaml:"stdsite"`
	StdInterval int    `yaml:"stdinterval"`
	StdSize     int    `yaml:"stdsize"`
	StdCount    int    `yaml:"stdcount"`
	OutSite     string `yaml:"outsite"`
	OutInterval int    `yaml:"outinterval"`
	OutCount    int    `yaml:"outcount"`
}

func (conf *Config) LoadConfig() {
	conf.ConfigFile = "/usr/local/etc/pingmon.conf"
	if _, err := os.Stat("pingmon.conf"); !os.IsNotExist(err) {
		conf.ConfigFile = "pingmon.conf"
	}
	f, err := os.ReadFile(conf.ConfigFile)
	if err != nil {
		log.Panicf("ERROR reading config file: %s\n", err)
	}
	if err = yaml.Unmarshal(f, conf); err != nil {
		log.Panicf("ERROR processing YAML: %s\n", err)
	}
}

func LogToDB(mode int, conf *Config, p *probing.Pinger, stats *probing.Statistics) error {

	insertQuery := `
        insert into PingLog (
            logTime
            , pingType
            , site
            , packetSize
            , packetCount
            , packetsSent
            , packetsReceived
            , minRTT
            , maxRTT
            , avgRTT
        ) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
    `
	pingType := "standard"
	if mode == MODE_OUTAGE {
		pingType = "outage"
	}

	db, err := sql.Open("sqlite3", conf.DBFile)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(insertQuery,
		time.Now(),
		pingType,
		stats.IPAddr.String(),
		p.Size,
		p.Count,
		stats.PacketsSent,
		stats.PacketsRecv,
		stats.MinRtt.Seconds(),
		stats.MaxRtt.Seconds(),
		stats.AvgRtt.Seconds(),
	)
	return err
}

func StandardPing(conf *Config) (mode int) {

	p, err := probing.NewPinger(conf.StdSite)
	if err != nil {
		log.Panicf("ERROR creating Standard Ping: %s\n", err)
	}
	p.Count = conf.StdCount
	p.Size = conf.StdSize
	p.Interval = 1 * time.Second
	p.Timeout = time.Duration(2*conf.StdCount) * time.Second

	if conf.Env == "test" {
		p.OnSend = func(pkt *probing.Packet) {
			log.Printf("Standard packet %d sent.\n", pkt.Seq)
		}
		p.OnRecv = func(pkt *probing.Packet) {
			log.Printf("Standard packet %d received: %.4f\n", pkt.Seq, pkt.Rtt.Seconds())
		}
	}
	p.OnFinish = func(stats *probing.Statistics) {
		mode = MODE_STANDARD
		if stats.PacketsRecv == 0 {
			mode = MODE_OUTAGE
			log.Println("SWITCHING to OUTAGE mode - all packets lost in the last batch.")
		}
		if err = LogToDB(mode, conf, p, stats); err != nil {
			log.Panicf("ERROR logging to DB: %s\n", err)
		}
		if conf.Env == "test" {
			log.Printf("Batch finished.\n")
		}
	}

	if err = p.Run(); err != nil {
		log.Panicf("ERROR Running Standard Ping: %s\n", err)
	}
	return mode
}

func OutagePing(conf *Config) (mode int) {

	p, err := probing.NewPinger(conf.OutSite)
	if err != nil {
		log.Panicf("ERROR creating Outage Ping: %s\n", err)
	}
	p.Count = conf.OutCount
	p.Size = 24
	p.Interval = time.Duration(conf.OutInterval) * time.Second
	p.Timeout = time.Duration(p.Count) * p.Interval

	if conf.Env == "test" {
		p.OnSend = func(pkt *probing.Packet) {
			log.Printf("Outage packet %d sent.\n", pkt.Seq)
		}
		p.OnRecv = func(pkt *probing.Packet) {
			log.Printf("Outage packet %d received: %.4f\n", pkt.Seq, pkt.Rtt.Seconds())
		}
	}
	p.OnFinish = func(stats *probing.Statistics) {
		mode = MODE_OUTAGE
		if stats.PacketsRecv == conf.OutCount {
			mode = MODE_STANDARD
			log.Println("SWITCHING to STANDARD mode - all packets received in the last batch.")
		}
		if err = LogToDB(mode, conf, p, stats); err != nil {
			log.Panicf("ERROR logging to DB: %s\n", err)
		}
		if conf.Env == "test" {
			log.Printf("Batch finished.\n")
		}
	}

	if err = p.Run(); err != nil {
		log.Panicf("ERROR running Outage Ping: %s\n", err)
	}

	return mode
}

func main() {
	var conf Config

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	defer func() {
		signal.Stop(sigChan)
	}()

	conf.LoadConfig()
	log.Printf("STARTING pingmon...\n\tSITE: %s\n\tCONFIG: %s\n\tDB: %s\n", conf.StdSite, conf.ConfigFile, conf.DBFile)

	go func() {
		for {
			select {
			case s := <-sigChan:
				switch s {
				case syscall.SIGINT, syscall.SIGTERM:
					log.Printf("EXITING - received %s.", s.String())
					os.Exit(0)
				case syscall.SIGHUP:
					log.Printf("RELOADING CONFIG")
					conf.LoadConfig()
				}
			}
		}
	}()

	mode := StandardPing(&conf)
	for {
		switch mode {
		case MODE_STANDARD:
			if conf.Env == "test" {
				log.Printf("Waiting %d seconds for next standard batch\n", conf.StdInterval)
			}
			time.Sleep(time.Duration(conf.StdInterval) * time.Second)
			mode = StandardPing(&conf)
		case MODE_OUTAGE:
			mode = OutagePing(&conf)
		}
	}
}
