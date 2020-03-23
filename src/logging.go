package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"time"
)

var gclog *GcLogger

const (
	logTimeFmt   = "2006/01/02 15:04:05 "
	logFileFlags = os.O_APPEND | os.O_CREATE | os.O_RDWR
	lAccessLog   = 1 << iota
	lErrorLog
	lStaffLog
	lStdLog
	lFatal
)

type GcLogger struct {
	accessFile *os.File
	errorFile  *os.File
	staffFile  *os.File
}

func (gcl *GcLogger) selectLogs(flags int) []*os.File {
	var logs []*os.File
	if flags&lAccessLog > 0 {
		logs = append(logs, gcl.accessFile)
	}
	if flags&lErrorLog > 0 {
		logs = append(logs, gcl.accessFile)
	}
	if flags&lStaffLog > 0 {
		logs = append(logs, gcl.accessFile)
	}
	if flags&lStdLog > 0 {
		logs = append(logs, os.Stdout)
	}
	return logs
}

func (gcl *GcLogger) getPrefix() string {
	prefix := time.Now().Format(logTimeFmt)
	_, file, line, _ := runtime.Caller(2)
	prefix += fmt.Sprint(file, ":", line, ": ")

	return prefix
}

func (gcl *GcLogger) Print(flags int, v ...interface{}) string {
	str := fmt.Sprint(v...)
	logs := gcl.selectLogs(flags)
	for _, l := range logs {
		if l == os.Stdout {
			io.WriteString(l, str+"\n")
		} else {
			io.WriteString(l, gcl.getPrefix()+str+"\n")
		}
		if flags&lFatal > 0 {
			os.Exit(1)
		}
	}
	return str
}

func (gcl *GcLogger) Printf(flags int, format string, v ...interface{}) string {
	str := fmt.Sprintf(format, v...)
	logs := gcl.selectLogs(flags)
	for _, l := range logs {
		if l == os.Stdout {
			io.WriteString(l, str+"\n")
		} else {
			io.WriteString(l, gcl.getPrefix()+str+"\n")
		}
		if flags&lFatal > 0 {
			os.Exit(1)
		}
	}
	return str
}

func (gcl *GcLogger) Println(flags int, v ...interface{}) string {
	str := fmt.Sprintln(v...)
	logs := gcl.selectLogs(flags)
	for _, l := range logs {
		if l == os.Stdout {
			io.WriteString(l, str+"\n")
		} else {
			io.WriteString(l, gcl.getPrefix()+str+"\n")
		}
		if flags&lFatal > 0 {
			defer os.Exit(1)
		}
	}
	return str
}

func (gcl *GcLogger) Close() {
	gcl.accessFile.Close()
	gcl.errorFile.Close()
	gcl.staffFile.Close()
}

func initLogs(accessLogPath, errorLogPath, staffLogPath string) *GcLogger {
	var gcl GcLogger
	var err error
	if gcl.accessFile, err = os.OpenFile(accessLogPath, logFileFlags, 0777); err != nil {
		fmt.Println("Error loading " + accessLogPath + ": " + err.Error())
		os.Exit(1)
	}
	if gcl.errorFile, err = os.OpenFile(errorLogPath, logFileFlags, 0777); err != nil {
		fmt.Println("Error loading " + errorLogPath + ": " + err.Error())
		os.Exit(1)
	}
	if gcl.staffFile, err = os.OpenFile(staffLogPath, logFileFlags, 0777); err != nil {
		fmt.Println("Error loading " + staffLogPath + ": " + err.Error())
		os.Exit(1)
	}
	return &gcl
}
