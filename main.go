package main

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/coreos/go-systemd/journal"
	"github.com/fsnotify/fsnotify"
)

func main() {
	paths := os.Args[1:]
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "expecting path(s) to scan")
		os.Exit(2)
	}

	if err := run(paths); err != nil {
		fmt.Fprintln(os.Stderr, "failed to start up: ", err)
		os.Exit(1)
	}
}

const (
	// ScanDelay is how long we wait for file access to be quiescent before
	// initiating a scan.
	ScanDelay = 2 * time.Second
)

// timers map filenames to callbacks that actually perform a scan. The idea is
// to wait for 2s of quiescence before initiating the scan, in case the file is
// in the process of being modified.
var timers struct {
	lock sync.Mutex
	t    map[string]*time.Timer
}

// run is the primary runtime function
func run(paths []string) error {
	timers.t = make(map[string]*time.Timer)

	var err error
	if successTpl, err = template.New("success").Parse(successTplT); err != nil {
		return err
	}
	if errorTpl, err = template.New("error").Parse(errorTplT); err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	for _, path := range paths {
		if err = watcher.Add(path); err != nil {
			return err
		}
	}

	go func() {
		if err := <-watcher.Errors; err != nil {
			fmt.Fprintln(os.Stderr, "fsnotify error: ", err)
			os.Exit(1)
		}
	}()

	for ev := range watcher.Events {
		localName := ev.Name // don't close over the loop var

		timers.lock.Lock()
		if tmr, already := timers.t[localName]; already {
			// cancel existing timer
			tmr.Stop()
		}

		// start new timer waiting for file access to be quiescent
		timers.t[localName] = time.AfterFunc(ScanDelay, func() {
			scan(localName)
			timers.lock.Lock()
			delete(timers.t, localName)
			timers.lock.Unlock()
		})
		timers.lock.Unlock()
	}
	return nil
}

// scan a file and report the result via dialog.
func scan(name string) {
	jvars := map[string]string{
		"CLAMSCAN_FNAME": name,
	}

	// fsnotify will trigger on removed files, symlinks, directories etc.
	// so filter out those cases
	st, err := os.Lstat(name)
	switch {
	case os.IsNotExist(err):
		journal.Send(fmt.Sprintf("not executing clamscan on %q "+
			"as it was removed", name), journal.PriDebug, jvars)
		return

	case !st.Mode().IsRegular():
		journal.Send(fmt.Sprintf("not executing clamscan on %q "+
			"as it is mode %v", name, st.Mode()),
			journal.PriDebug, jvars)
		return
	}

	jvars["CLAMSCAN_FILESIZE"] = fmt.Sprintf("%d", st.Size())
	journal.Send(fmt.Sprintf("executing clamscan on %q "+
		"(%d bytes)", name, st.Size()),
		journal.PriDebug, jvars)

	clam := exec.Command("clamdscan", name)
	clamOP, err := clam.CombinedOutput()

	status := "success"
	priority := journal.PriNotice
	if err != nil {
		status = err.Error()
		priority = journal.PriAlert
	}
	jvars["CLAMSCAN_STATUS"] = status
	jvars["CLAMSCAN_OUTPUT"] = string(clamOP)
	journal.Send(fmt.Sprintf("clamscan executed on %q with status: %s",
		name, status), priority, jvars)
	dialog(clamOP, err)
}
