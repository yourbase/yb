// Copyright 2021 YourBase Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//		 https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/yourbase/commons/envvar"
	"github.com/yourbase/yb/internal/config"
	"go.opentelemetry.io/otel/api/trace"
	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"
	"zombiezen.com/go/log"
)

const longTimeFormat = "15:04:05 MST"

type logger struct {
	color      bool
	showLevels bool

	mu  sync.Mutex
	buf []byte
}

var logInitOnce sync.Once

func initLog(cfg config.Getter, showDebug bool) {
	logInitOnce.Do(func() {
		log.SetDefault(&log.LevelFilter{
			Min: configuredLogLevel(cfg, showDebug),
			Output: &logger{
				color:      colorLogs(),
				showLevels: showLogLevels(cfg),
			},
		})
	})
}

func (l *logger) Log(ctx context.Context, entry log.Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.buf = l.buf[:0]
	if l.showLevels {
		if l.color {
			switch {
			case entry.Level >= log.Error:
				// Red text
				l.buf = append(l.buf, "\x1b[31m"...)
			case entry.Level >= log.Warn:
				// Yellow text
				l.buf = append(l.buf, "\x1b[33m"...)
			default:
				// Cyan text
				l.buf = append(l.buf, "\x1b[36m"...)
			}
		}
		switch {
		case entry.Level >= log.Error:
			l.buf = append(l.buf, "ERROR"...)
		case entry.Level >= log.Warn:
			l.buf = append(l.buf, "WARN"...)
		case entry.Level >= log.Info:
			l.buf = append(l.buf, "INFO"...)
		default:
			l.buf = append(l.buf, "DEBUG"...)
		}
		if l.color {
			l.buf = append(l.buf, "\x1b[0m"...)
		}
		l.buf = append(l.buf, ' ')
	}
	l.buf = append(l.buf, entry.Msg...)
	l.buf = append(l.buf, '\n')
	os.Stderr.Write(l.buf)
}

func (l *logger) LogEnabled(entry log.Entry) bool {
	return true
}

func configuredLogLevel(cfg config.Getter, showDebug bool) log.Level {
	if showDebug {
		return log.Debug
	}
	l := config.Get(cfg, "defaults", "log-level")
	switch strings.ToLower(l) {
	case "debug":
		return log.Debug
	case "warn", "warning":
		return log.Warn
	case "error":
		return log.Error
	}
	return log.Info
}

func colorLogs() bool {
	b, _ := strconv.ParseBool(envvar.Get("CLICOLOR", "1"))
	return b
}

func showLogLevels(cfg config.Getter) bool {
	out := config.Get(cfg, "defaults", "no-pretty-output")
	if out != "" {
		b, _ := strconv.ParseBool(out)
		return !b
	}
	return !envvar.Bool("YB_NO_PRETTY_OUTPUT")
}

// A traceSink records spans in memory. The zero value is an empty sink.
type traceSink struct {
	mu        sync.Mutex
	rootSpans []*exporttrace.SpanData
	children  map[trace.SpanID][]*exporttrace.SpanData
}

// ExportSpan saves the trace span. It is safe to be called concurrently.
func (sink *traceSink) ExportSpan(_ context.Context, span *exporttrace.SpanData) {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if !span.ParentSpanID.IsValid() {
		sink.rootSpans = append(sink.rootSpans, span)
		return
	}
	if sink.children == nil {
		sink.children = make(map[trace.SpanID][]*exporttrace.SpanData)
	}
	sink.children[span.ParentSpanID] = append(sink.children[span.ParentSpanID], span)
}

const (
	traceDumpStartWidth   = 14
	traceDumpEndWidth     = 14
	traceDumpElapsedWidth = 14
)

// dump formats the recorded traces as a hierarchial table of spans in the order
// received. It is safe to call concurrently, including with ExportSpan.
func (sink *traceSink) dump() string {
	sb := new(strings.Builder)
	fmt.Fprintf(sb, "%-*s %-*s %-*s\n",
		traceDumpStartWidth, "Start",
		traceDumpEndWidth, "End",
		traceDumpElapsedWidth, "Elapsed",
	)
	sink.mu.Lock()
	sink.dumpLocked(sb, trace.SpanID{}, 0)
	sink.mu.Unlock()
	return sb.String()
}

func (sink *traceSink) dumpLocked(sb *strings.Builder, parent trace.SpanID, depth int) {
	const indent = "  "
	list := sink.rootSpans
	if parent.IsValid() {
		list = sink.children[parent]
	}
	if depth >= 3 {
		if len(list) > 0 {
			writeSpaces(sb, traceDumpStartWidth+traceDumpEndWidth+traceDumpElapsedWidth+3)
			for i := 0; i < depth; i++ {
				sb.WriteString(indent)
			}
			sb.WriteString("...\n")
		}
		return
	}
	for _, span := range list {
		elapsed := span.EndTime.Sub(span.StartTime)
		fmt.Fprintf(sb, "%-*s %-*s %*.3fs %s\n",
			traceDumpStartWidth, span.StartTime.Format(longTimeFormat),
			traceDumpEndWidth, span.EndTime.Format(longTimeFormat),
			traceDumpElapsedWidth-1, elapsed.Seconds(),
			strings.Repeat(indent, depth)+span.Name,
		)
		sink.dumpLocked(sb, span.SpanContext.SpanID, depth+1)
	}
}

func writeSpaces(w io.ByteWriter, n int) {
	for i := 0; i < n; i++ {
		w.WriteByte(' ')
	}
}
