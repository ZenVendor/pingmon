create table PingLog (
    logTime datetime not null,
    pingType text not null,
    site text not null,
    packetSize int not null,
    packetCount int not null,
    packetsSent int not null,
    packetsReceived int not null,
    minRTT float not null,
    maxRTT float not null,
    avgRTT float not null
);
