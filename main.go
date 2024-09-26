package main

import (
	"database/sql"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	probing "github.com/prometheus-community/pro-bing"
)

const (
    MODE_STANDARD = iota
    MODE_OUTAGE
)

func StandardPing(site, insertQuery string, db *sql.DB, l *log.Logger) (mode int) {

    pStd, err := probing.NewPinger(site)
    if err != nil {
        l.Fatal(err)
    }
    pStd.Count = 3
    pStd.Size = 1000
    pStd.Timeout = 5 * time.Second

    pStd.OnFinish = func(stats *probing.Statistics) {
        if stats.PacketsRecv == 0 {
            mode = MODE_OUTAGE
            l.Println("No response - switching to OUTAGE mode.")
        }
        if stats.PacketsRecv > 0 {
            mode = MODE_STANDARD
            _, err = db.Exec(insertQuery, 
                time.Now(),
                "standard",
                stats.IPAddr.IP, 
                pStd.Size, 
                pStd.Count, 
                stats.PacketsSent, 
                stats.PacketsRecv, 
                stats.MinRtt.Milliseconds(),
                stats.MaxRtt.Milliseconds(),
                stats.AvgRtt.Milliseconds(),
            )
            if err != nil {
                l.Fatal(err)
            }
        }
    }

    err = pStd.Run()
    if err != nil {
        log.Fatal(err)
    }
    return mode
}

func OutagePing(site, insertQuery string, db *sql.DB, l *log.Logger) (mode int) {

    pOut, err := probing.NewPinger(site)
    if err != nil {
        l.Fatal(err)
    }
    pOut.Count = 5
    pOut.Size = 24
    pOut.Interval = 3 * time.Second
    pOut.Timeout = 15 * time.Second

    pOut.OnFinish = func(stats *probing.Statistics) {
        if stats.PacketsRecv < 5 {
            mode = MODE_OUTAGE

            _, err = db.Exec(insertQuery, 
                time.Now(),
                "outage",
                stats.IPAddr.IP, 
                pOut.Size, 
                pOut.Count, 
                stats.PacketsSent, 
                stats.PacketsRecv, 
                stats.MinRtt.Milliseconds(),
                stats.MaxRtt.Milliseconds(),
                stats.AvgRtt.Milliseconds(),
            )
            if err != nil {
                l.Fatal(err)
            }
        }
        if stats.PacketsRecv == 5 {
            mode = MODE_STANDARD
            l.Println("All packets received - returning to STANDARD mode")

            _, err = db.Exec(insertQuery, 
                time.Now(),
                "outage",
                stats.IPAddr.IP, 
                pOut.Size, 
                pOut.Count, 
                stats.PacketsSent, 
                stats.PacketsRecv, 
                stats.MinRtt.Milliseconds(),
                stats.MaxRtt.Milliseconds(),
                stats.AvgRtt.Milliseconds(),
            )
            if err != nil {
                l.Fatal(err)
            }
        }
    }
    err = pOut.Run()
    if err != nil {
        l.Fatal(err)
    }
    return mode
}

var db *sql.DB

func main() {

    envs, err := godotenv.Read()
    if err != nil {
        log.Fatal(err)
    }

    logFile, err := os.OpenFile(envs["LOGFILE"], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatal(err)
    }
    defer logFile.Close()

    l := log.New(logFile, "pingmon: ", log.LstdFlags|log.Lshortfile)

    site := envs["SITE"]
    interval, _ := strconv.Atoi(envs["STDINTERVAL"])
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

    db, err := sql.Open("sqlite3", envs["DBFILE"])
    if err != nil {
        l.Fatal(err)
    }
    defer db.Close()

    mode := StandardPing(site, insertQuery, db, l)

    for {
        switch mode {
        case MODE_STANDARD:
            time.Sleep(time.Duration(interval) * time.Second)
            mode = StandardPing(site, insertQuery, db, l)
        case MODE_OUTAGE: 
            mode = OutagePing(site, insertQuery, db, l)
        }
    }

}
