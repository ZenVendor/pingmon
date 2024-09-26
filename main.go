package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	probing "github.com/prometheus-community/pro-bing"
)

const (
    MODE_STANDARD = iota
    MODE_OUTAGE
)

func LogToDB(dbFile string, mode int, p *probing.Pinger, stats *probing.Statistics) error {

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

    db, err := sql.Open("sqlite3", dbFile)
    if err != nil {
        return err
    }
    defer db.Close()

    _, err = db.Exec(insertQuery, 
        time.Now(),
        pingType,
        stats.IPAddr.IP, 
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

func StandardPing(site, dbFile string, l *log.Logger) (mode int) {

    p, err := probing.NewPinger(site)
    if err != nil {
        l.Fatal(err)
    }
    p.Count = 3
    p.Size = 1000
    p.Timeout = 5 * time.Second

    p.OnFinish = func(stats *probing.Statistics) {
        mode = MODE_STANDARD
        if stats.PacketsRecv == 0 {
            mode = MODE_OUTAGE
            l.Println("SWITCHING to \033[38;5;9mOUTAGE\033[0m mode - all packets lost in the last batch.")
        }
        if err = LogToDB(dbFile, mode, p, stats); err != nil {
            l.Fatal(err)
        }
    }

    if err = p.Run(); err != nil {
        l.Fatal(err)
    }
    return mode
}

func OutagePing(site, dbFile string, l *log.Logger) (mode int) {

    p, err := probing.NewPinger(site)
    if err != nil {
        l.Fatal(err)
    }
    p.Count = 5
    p.Size = 24
    p.Interval = 3 * time.Second
    p.Timeout = 15 * time.Second

    p.OnFinish = func(stats *probing.Statistics) {
        mode = MODE_OUTAGE
        if stats.PacketsRecv == 5 {
            mode = MODE_STANDARD
            l.Println("SWITCHING to \033[38;5;40mSTANDARD\033[0m mode - all packets received in the last batch.")
        }
        if err = LogToDB(dbFile, mode, p, stats); err != nil {
            l.Fatal(err)
        }
    }

    if err = p.Run(); err != nil {
        l.Fatal(err)
    }

    return mode
}

func main() {

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

    envs, err := godotenv.Read()
    if err != nil {
        log.Fatal(err)
    }

    logFile, err := os.OpenFile(envs["LOGFILE"], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatal(err)
    }
    defer logFile.Close()

    l := log.New(logFile, "pingmon: ", log.LstdFlags)

    site := envs["SITE"]
    dbFile := envs["DBFILE"]
    interval, _ := strconv.Atoi(envs["STDINTERVAL"])

    l.Printf("STARTING pingmon...\n\tSITE: %s\n\tDBFILE: %s\n", site, dbFile)

    mode := StandardPing(site, dbFile, l)
    for {
        select {
        case <-sigChan:
            l.Printf("EXITING after signal\n")
            return
        case<-time.After(100 * time.Millisecond):
            switch mode {
            case MODE_STANDARD:
                time.Sleep(time.Duration(interval) * time.Second)
                mode = StandardPing(site, dbFile, l)
            case MODE_OUTAGE: 
                mode = OutagePing(site, dbFile, l)
            }
        }
    }
}
