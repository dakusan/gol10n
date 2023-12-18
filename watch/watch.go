//The interface to watch for changes
//go:build !gol10n_read_compiled_only

// Package watch contains the watch.Execute function called by the main command line interface
package watch

import (
	"errors"
	"fmt"
	"github.com/dakusan/gol10n/execute"
	"github.com/dakusan/gol10n/translate"
	"github.com/fsnotify/fsnotify"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ReturnData is the data that is returned through a channel from watch.Execute when it processes files
type ReturnData struct {
	Type    ReturnType
	Files   execute.ProcessedFileList //Only on ReturnType=WR_ProcessedDirectory
	Err     error                     //Only on ReturnType=WR_ProcessedDirectory or WR_ProcessedFile or WR_ErroredOut
	Message string                    //Only on ReturnType=WR_Message or WR_ProcessedFile
}

type ReturnType int

//goland:noinspection GoSnakeCaseUsage
const (
	WR_Message            ReturnType = iota //An informative message is being sent
	WR_ProcessedDirectory                   //Directory() was called due to initialization or default language update
	WR_ProcessedFile                        //A single file was updated. Message contains the filename. Error is filled on error.
	WR_ErroredOut                           //The watch could not be started or has closed
	WR_CloseRequested                       //Process close was requested
)

// Execute processes all files in the InputPath directory.
//
// It continually watches the directory for relevant changes in its own goroutine, and only processes and updates the necessary files when a change is detected.
func Execute(settings *execute.ProcessSettings) <-chan ReturnData {
	ret := make(chan ReturnData, 10)
	go execWatchReal(settings, ret)
	return ret
}

func execWatchReal(settings *execute.ProcessSettings, ret chan<- ReturnData) {
	//Send a message ReturnData
	sendMessage := func(message string) {
		ret <- ReturnData{WR_Message, nil, nil, message}
	}

	//Create the watcher
	var watcher *fsnotify.Watcher
	if _watcher, err := fsnotify.NewWatcher(); err != nil {
		ret <- ReturnData{WR_ErroredOut, nil, err, ""}
		return
	} else {
		watcher = _watcher
	}
	defer func() { _ = watcher.Close() }()
	if err := watcher.Add(settings.InputPath); err != nil {
		ret <- ReturnData{WR_ErroredOut, nil, err, ""}
		return
	}

	//Execute the primary Directory() function first before we start watching
	{
		langs, err := settings.Directory()
		ret <- ReturnData{WR_ProcessedDirectory, langs, err, ""}
	}

	//Keeps a list of file changes that have happened within the last $timeoutWatch
	//These are cancelled if a duplicate event occurs within the timeout
	const timeoutWatch = time.Millisecond * 100
	recentWatches := make(map[string]*bool) //If the bool pointer is set to true then the event is cancelled
	var recentWatchesMutex sync.RWMutex

	//Handle os shutdown signal
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt)
	signal.Notify(shutdownSignal, syscall.SIGTERM)

	//Execute the watcher
	sendMessage("Initiating watch")
	for {
		select {
		//Return error message
		case err, ok := <-watcher.Errors:
			if !ok {
				ret <- ReturnData{WR_ErroredOut, nil, errors.New("Watcher was closed out"), ""}
				return
			}
			sendMessage("Watcher sent an error: " + err.Error())
		//Process an event
		case event, ok := <-watcher.Events:
			//Check for valid event
			var langIdent string
			fName := strings.ReplaceAll(event.Name, "\\", "/")
			if !ok {
				ret <- ReturnData{WR_ErroredOut, nil, errors.New("Watcher was closed out"), ""}
				return
			} else if !strings.HasPrefix(fName, settings.InputPath) {
				sendMessage(fmt.Sprintf("Changed file “%s” did not have the correct input path “%s”", fName, settings.InputPath))
				continue
			}
			fName = fName[len(settings.InputPath):]
			if !event.Has(fsnotify.Write | fsnotify.Create) { //Ignore Rename and Delete since file no longer exists
				continue
			} else if dotLoc := strings.LastIndexByte(fName, '.'); dotLoc == -1 {
				continue
			} else if ext := fName[dotLoc+1:]; ext != execute.YAML_Extension && ext != execute.JSON_Extension {
				continue
			} else {
				langIdent = fName[0:dotLoc]
			}

			//Wait for $timeoutWatch before executing. If a duplicate event comes in, erase the previous event
			go func() {
				//If the event already exists then mark it as cancelled
				recentWatchesMutex.Lock()
				eventKey := event.String()
				if b, exists := recentWatches[eventKey]; exists {
					*b = true
				}

				//Add a cancellable bool for this event
				isCancelled := false
				recentWatches[eventKey] = &isCancelled
				recentWatchesMutex.Unlock()

				//Wait for the timeout to check and see if cancelled
				time.Sleep(timeoutWatch)

				//See if the event was cancelled and exit if so
				recentWatchesMutex.Lock()
				if isCancelled {
					recentWatchesMutex.Unlock()
					return
				}

				//Remove self from the recent watches list
				delete(recentWatches, eventKey)
				recentWatchesMutex.Unlock()

				//Send message about change and process the file
				sendMessage(fmt.Sprintf("%s: Change (%s) occurred on “%s”", time.Now().Format("2006-01-02 15:04:05"), event.Op.String(), fName))
				processFile(langIdent, fName, settings, ret)
			}()
		case <-shutdownSignal:
			ret <- ReturnData{WR_CloseRequested, nil, nil, ""}
			return
		}
	}
}

func processFile(langIdent, fName string, settings *execute.ProcessSettings, ret chan<- ReturnData) {
	//If this is the default language then clear the dictionary and run a full Directory() call
	if langIdent == settings.DefaultLanguage {
		translate.LanguageFile(translate.LF_YAML).ClearCurrentDictionary()
		langs, err := settings.Directory()
		ret <- ReturnData{WR_ProcessedDirectory, langs, err, ""}
		return
	}

	//Process the file normally
	//While this could cause a problem if there were multiple text files with the same language identifier, I don't think that’s an edge case I really need to worry about right here
	ret <- ReturnData{WR_ProcessedFile, nil, settings.FileCompileOnly(langIdent), fName}
}
