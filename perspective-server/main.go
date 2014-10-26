// Perspective: Graphing library for quality control in event-driven systems

// Copyright (C) 2014 Christian Paro <christian.paro@gmail.com>,
//                                   <cparo@digitalocean.com>

// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License version 2 as published by the
// Free Software Foundation.

// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS
// FOR A PARTICULAR PURPOSE. See the GNU General Public License for more
// details.

// You should have received a copy of the GNU General Public License along with
// this program. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"github.com/cparo/perspective"
	"github.com/cparo/perspective/feeds"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Mapping of action names to handler functions:
var handlers = make(map[string]func(io.Writer, *requestOptions))

// Mapping of data-source paths to loaded data:
var sources = make(map[string]*[]feeds.EventData)

// Options and arguments:
type requestOptions struct {
	typeFilter int     // Event type to filter for, if non-negative.
	tA         int     // Lower limit of time range to be visualized.
	tΩ         int     // Upper limit of time range to be visualized.
	xGrid      int     // Number of horizontal grid divisions.
	yLog2      float64 // Number of pixels over which elapsed times double.
	w          int     // Visualization width, in pixels.
	h          int     // Visualization height, in pixels.
	colors     int     // The number of color steps before saturation.
	iPath      string  // Filesystem path for input.
}

func init() {

	handlers["vis-error-stack"] = func(out io.Writer, r *requestOptions) {
		visualize(perspective.NewErrorStack(r.w, r.h), out, r)
	}

	handlers["vis-histogram"] = func(out io.Writer, r *requestOptions) {
		visualize(perspective.NewHistogram(r.w, r.h, r.yLog2), out, r)
	}

	handlers["vis-rolling-stack"] = func(out io.Writer, r *requestOptions) {
		visualize(perspective.NewRollingStack(r.w, r.h, r.tA, r.tΩ), out, r)
	}

	handlers["vis-scatter"] = func(out io.Writer, r *requestOptions) {
		visualize(
			perspective.NewScatter(
				r.w, r.h, r.tΩ, r.tA, r.yLog2, r.colors, r.xGrid),
			out,
			r)
	}

	handlers["vis-status-stack"] = func(out io.Writer, r *requestOptions) {
		visualize(perspective.NewStatusStack(r.w, r.h), out, r)
	}

	handlers["vis-sweep"] = func(out io.Writer, r *requestOptions) {
		visualize(
			perspective.NewSweep(
				r.w, r.h, r.tA, r.tΩ, r.yLog2, r.colors, r.xGrid),
			out,
			r)
	}

	handlers["vis-wave"] = func(out io.Writer, r *requestOptions) {
		visualize(perspective.NewWave(r.w, r.h, r.tA, r.tΩ), out, r)
	}

	handlers["vis-wave-sorted"] = func(out io.Writer, r *requestOptions) {
		visualize(perspective.NewSortedWave(r.w, r.h, r.tA, r.tΩ), out, r)
	}
}

func intOpt(values url.Values, name string, defaultValue int) int {
	strValue := values.Get(name)
	if strValue == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(strValue)
	if err != nil {
		logMalformedOption(name, strValue)
		return defaultValue
	}
	return intValue
}

func f64Opt(values url.Values, name string, defaultValue float64) float64 {
	strValue := values.Get(name)
	if strValue == "" {
		return defaultValue
	}
	f64Value, err := strconv.ParseFloat(strValue, 64)
	if err != nil {
		logMalformedOption(name, strValue)
		return defaultValue
	}
	return f64Value
}

func logFileLoad(path string) {
	log.Printf("Loading data from file: \"%s\"\n", path)
}

func logMalformedOption(name string, value string) {
	log.Printf("Malformed option: %s = \"%s\"\n", name, value)
}

func main() {
	http.HandleFunc("/", responder)
	http.ListenAndServe(":8080", nil)
}

func responder(response http.ResponseWriter, request *http.Request) {

	// Parse options, using the same defaults as are used by the CLI interface
	// where options are misisng or malformed:
	values := request.URL.Query()
	options := &requestOptions{
		intOpt(values, "event-type", -1),
		intOpt(values, "min-time", 0),
		intOpt(values, "max-time", int(time.Now().Unix())),
		intOpt(values, "x-grid", 0),
		f64Opt(values, "run-time-scale", 16),
		intOpt(values, "width", 256),
		intOpt(values, "height", 256),
		intOpt(values, "color-steps", 1),
		"/home/cparo/Devel/compact_event_dump_creates_only.dat"}
	// TODO: REPLACE HARD-CODED INPUT PATH ABOVE

	action := request.URL.Path[1:]
	if handler, exists := handlers[action]; exists {
		handler(response, options)
	} else {
		log.Printf("Unrecognized action: %s\n", action)
		http.NotFound(response, request)
	}
}

func visualize(v perspective.Visualizer, out io.Writer, r *requestOptions) {

	// Load the event data if it is not already loaded.
	// TODO: Some re-work will be needed here to do this in a thread-safe
	//       manner before allowing this server to concurrently handle multiple
	//       requests. Essentially, we will want to make a worker thread which
	//       handles loading of and access to these mapped-file pointers - and
	//       then have our HTTP request handlers send in requests for mapped
	//       pointers by path which will then be asynchronously returned by the
	//       worker (which can either process these requests sequentially or
	//       lock on a per-path basis - in either case safeguarding against
	//       race conditions). Practically speaking this is probably a non-issue
	//       given the narrow time windows involved and invariant nature of the
	//       logs behind these maps once generated - but there is no built-in
	//       provision for safe concurrent manipulation of Go's maps themselves,
	//       so proper defensive practice would be to make this unlikely issue
	//       an impossible one.
	path := r.iPath
	if _, loaded := sources[path]; !loaded {
		logFileLoad(path)
		sources[path] = feeds.MapBinLogFile(path)
	}

	feeds.GeneratePNGFromBinLog(sources[path], r.tA, r.tΩ, r.typeFilter, v, out)
}