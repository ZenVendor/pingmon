package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	probing "github.com/prometheus-community/pro-bing"
)


func main() {

    logFile, err := os.OpenFile("pingmon.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatal(err)
    }
    defer logFile.Close()

    l := log.New(logFile, "pingmon: ", log.LstdFlags|log.Lshortfile)

    insertQuery := "insert into PingLog (logTime, pingType, site, packetSize, packetCount, packetsSent, packetsReceived, minRTT, maxRTT, avgRTT, stdDevRTT) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);"
    site := "8.8.8.8"

    db, err := sql.Open("sqlite3", "pingmon.db")
    if err != nil {
        l.Fatal(err)
    }

    pinger, err := probing.NewPinger(site)
    if err != nil {
        l.Fatal(err)
    }
    pinger.Count = 5
    pinger.Size = 1000
    pinger.OnFinish = func(stats *probing.Statistics) {
        //l.Printf("Transmitted: %d, Received: %d, Lost: %v%%\n", stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
        l.Printf("Round trip min/max/avg/stdev: %v / %v / %v / %v\n", stats.MinRtt, stats.MaxRtt, stats.AvgRtt, stats.StdDevRtt)
        _, err = db.Exec(insertQuery, 
            time.Now(),
            "standard",
            site, 
            pinger.Size, 
            pinger.Count, 
            stats.PacketsSent, 
            stats.PacketsRecv, 
            stats.MinRtt.Milliseconds(),
            stats.MaxRtt.Milliseconds(),
            stats.AvgRtt.Milliseconds(),
            stats.StdDevRtt.Milliseconds(),
        )
        if err != nil {
            l.Fatal(err)
        }
    }
    err = pinger.Run()
    if err != nil {
        l.Fatal(err)
    }
}
