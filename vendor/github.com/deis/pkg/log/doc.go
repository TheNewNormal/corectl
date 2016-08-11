// Package log provides functionality to output human readable, colorful test to STDOUT and STDERR. It's best used for programs, such as CLI apps, that write output to people rather than machines. It is not intended for logging to log aggregators or other systems. It takes advantage of the github.com/deis/pkg/prettyprint to provide colorful output.
//
// This package provides global functions for use as well as a 'Logger' struct that you can instantiate at-will for customized logging. All global funcs operate on a DefaultLogger, which is pre-configured to log to os.Stdout and os.Stderr, with debug logs turned off.
//
// Example usage of global functions:
//
//  import "github.com/deis/pkg/log"
//  log.Info("Hello Gophers!") // equivalent of log.DefaultLogger.Info("hello gophers!")
//  log.Debug("log.DefaultLogger initializes with debug logs turned off, so you can't see me!")
//  log.DefaultLogger.SetDebug(true)
//  log.Debug("Now that we turned debug logs on, you can see me now!")
//
// Example usage of instantiating an individual logger:
//
//  // create a new logger that sends all stderr logs to /dev/null, and turns on debug logs
//  logger := log.NewLogger(os.Stdout, iouitl.Discard, true)
//  log.Debug("Hello Gophers!")
package log
