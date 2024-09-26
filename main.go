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
	DBFile      string `yaml:"dbfile"`
	LogFile     string `yaml:"logfile"`
	StdSite     string `yaml:"stdsite"`
	StdInterval int    `yaml:"stdinterval"`
	StdSize     int    `yaml:"stdsize"`
	StdCount    int    `yaml:"stdcount"`
	OutSite     string `yaml:"outsite"`
	OutInterval int    `yaml:"outinterval"`
	OutCount    int    `yaml:"outcount"`
}

func (conf *Config) LogToDB(mode int, p *probing.Pinger, stats *probing.Statistics) error {

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

func (conf *Config) StandardPing(l *log.Logger) (mode int) {

	p, err := probing.NewPinger(conf.StdSite)
	if err != nil {
		l.Fatal(err)
	}
	p.Count = conf.StdCount
	p.Size = conf.StdSize
	p.Interval = 1 * time.Second
	p.Timeout = time.Duration(2*conf.StdCount) * time.Second

	p.OnFinish = func(stats *probing.Statistics) {
		mode = MODE_STANDARD
		if stats.PacketsRecv == 0 {
			mode = MODE_OUTAGE
			l.Println("SWITCHING to \033[38;5;9mOUTAGE\033[0m mode - all packets lost in the last batch.")
		}
		if err = conf.LogToDB(mode, p, stats); err != nil {
			l.Fatal(err)
		}
	}

	if err = p.Run(); err != nil {
		l.Fatal(err)
	}
	return mode
}

func (conf *Config) OutagePing(l *log.Logger) (mode int) {

	p, err := probing.NewPinger(conf.OutSite)
	if err != nil {
		l.Fatal(err)
	}
	p.Count = conf.OutCount
	p.Size = 24
	p.Interval = time.Duration(conf.OutInterval) * time.Second
	p.Timeout = time.Duration(p.Count) * p.Interval

	p.OnFinish = func(stats *probing.Statistics) {
		mode = MODE_OUTAGE
		if stats.PacketsRecv == conf.OutCount {
			mode = MODE_STANDARD
			l.Println("SWITCHING to \033[38;5;40mSTANDARD\033[0m mode - all packets received in the last batch.")
		}
		if err = conf.LogToDB(mode, p, stats); err != nil {
			l.Fatal(err)
		}
	}

	if err = p.Run(); err != nil {
		l.Fatal(err)
	}

	return mode
}

var conf Config

func main() {

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	conf.ConfigFile = "/usr/local/etc/pingmon.conf"

	f, err := os.ReadFile(conf.ConfigFile)
	if err != nil {
		log.Fatal(err)
	}
	if err = yaml.Unmarshal(f, &conf); err != nil {
		log.Fatal(err)
	}

	logFile, err := os.OpenFile(conf.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	l := log.New(logFile, "pingmon: ", log.LstdFlags)

	l.Printf("STARTING pingmon...\n\tSITE: %s\n\tCONFIG: %s\n\tDB: %s\n\tLOG: %s\n", conf.StdSite, conf.ConfigFile, conf.DBFile, conf.LogFile)

	mode := conf.StandardPing(l)
	for {
		select {
		case <-sigChan:
			l.Printf("EXITING after signal\n")
			return
		case <-time.After(100 * time.Millisecond):
			switch mode {
			case MODE_STANDARD:
				time.Sleep(time.Duration(conf.StdInterval) * time.Second)
				mode = conf.StandardPing(l)
			case MODE_OUTAGE:
				mode = conf.OutagePing(l)
			}
		}
	}
}
