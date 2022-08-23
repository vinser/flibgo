package rlog

import (
	"io"
	"log"
)

type Log struct {
	File *RotaryLog
	D    *log.Logger // debug
	I    *log.Logger // info
	E    *log.Logger // error
}

func NewLog(logFile, level string) *Log {
	var dw = io.Discard
	var iw = io.Discard
	var ew io.Writer

	fw, err := NewRotaryLog(logFile, 86400, 0, 0)
	if err != nil {
		log.Fatal(err)
	}
	switch level {
	case "D":
		dw = fw
		fallthrough
	case "I":
		iw = fw
		fallthrough
	case "E":
		ew = fw
	default:
		ew = fw
	}
	return &Log{
		File: fw,
		D:    log.New(dw, "DEBUG:\t", log.LstdFlags|log.Lshortfile),
		I:    log.New(iw, "INFO:\t", log.LstdFlags),
		E:    log.New(ew, "ERROR:\t ", log.LstdFlags|log.Lshortfile),
	}
}
