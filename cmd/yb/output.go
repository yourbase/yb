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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yourbase/yb/internal/config"
	"go.opentelemetry.io/otel/api/trace"
	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"
	"zombiezen.com/go/log"
)

const (
	shortTimeFormat = "15:04:05"
	longTimeFormat  = "15:04:05 MST"
)

// setupLogPrefix is the log prefix used with withLogPrefix when running a
// target's setup.
const setupLogPrefix = ".deps"

type logger struct {
	color termStyles

	mu  sync.Mutex
	buf []byte
}

var logInitOnce sync.Once

func initLog(cfg config.Getter, showDebug bool) {
	logInitOnce.Do(func() {
		log.SetDefault(&log.LevelFilter{
			Min: configuredLogLevel(cfg, showDebug),
			Output: &logger{
				color: termStylesFromEnv(),
			},
		})
	})
}

func (l *logger) Log(ctx context.Context, entry log.Entry) {
	logToStdout, _ := ctx.Value(logToStdoutKey{}).(bool)
	logToStdout = logToStdout && entry.Level < log.Warn
	var colorSequence string
	switch {
	case entry.Level < log.Info:
		colorSequence = l.color.debug()
	case entry.Level >= log.Error:
		colorSequence = l.color.failure()
	}
	prefix, _ := ctx.Value(logPrefixKey{}).(string)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.buf = l.buf[:0]
	l.buf = append(l.buf, colorSequence...)
	for _, line := range strings.Split(strings.TrimSuffix(entry.Msg, "\n"), "\n") {
		l.buf = appendLogPrefix(l.buf, entry.Time, prefix)
		switch {
		case entry.Level >= log.Error:
			l.buf = append(l.buf, "❌ "...)
		case entry.Level >= log.Warn:
			l.buf = append(l.buf, "⚠️ "...)
		}
		l.buf = append(l.buf, line...)
		l.buf = append(l.buf, '\n')
	}
	if colorSequence != "" {
		l.buf = append(l.buf, l.color.reset()...)
	}

	out := os.Stderr
	if logToStdout {
		out = os.Stdout
	}
	out.Write(l.buf)
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

type logToStdoutKey struct{}

func withStdoutLogs(parent context.Context) context.Context {
	if isPresent, _ := parent.Value(logToStdoutKey{}).(bool); isPresent {
		return parent
	}
	return context.WithValue(parent, logToStdoutKey{}, true)
}

type logPrefixKey struct{}

func withLogPrefix(parent context.Context, prefix string) context.Context {
	parentPrefix, _ := parent.Value(logPrefixKey{}).(string)
	return context.WithValue(parent, logPrefixKey{}, parentPrefix+prefix)
}

// linePrefixWriter prepends a timestamp and a prefix string to every line
// written to it and writes to an underlying writer.
type linePrefixWriter struct {
	dst    io.Writer
	prefix string
	buf    []byte
	wrote  bool
	now    func() time.Time
}

func newLinePrefixWriter(w io.Writer, prefix string) *linePrefixWriter {
	return &linePrefixWriter{
		dst:    w,
		prefix: prefix,
		now:    time.Now,
	}
}

// Write writes p to the underlying writer, inserting line prefixes as required.
// Write will issue at most one Write call per line on the underlying writer.
func (lp *linePrefixWriter) Write(p []byte) (int, error) {
	now := lp.now()
	origLen := len(p)
	for next := []byte(nil); len(p) > 0; p = next {
		line := p
		next = nil
		lineEnd := bytes.IndexByte(p, '\n')
		if lineEnd != -1 {
			line, next = p[:lineEnd+1], p[lineEnd+1:]
		}
		if lp.wrote {
			// Pass through rest of line if we already wrote the prefix.
			n, err := lp.dst.Write(line)
			lp.wrote = lineEnd == -1
			if err != nil {
				return origLen - (len(p) + n), err
			}
			continue
		}
		lp.buf = appendLogPrefix(lp.buf[:0], now, lp.prefix)
		lp.buf = append(lp.buf, line...)
		n, err := lp.dst.Write(lp.buf)
		lp.wrote = n > 0 && (n < len(lp.buf) || lineEnd == -1)
		if err != nil {
			n -= len(lp.buf) - len(line) // exclude prefix length
			return origLen - (len(p) + n), err
		}
	}
	return origLen, nil
}

// appendLogPrefix formats the given timestamp and label and appends the result
// to dst.
func appendLogPrefix(dst []byte, t time.Time, prefix string) []byte {
	if prefix == "" {
		return dst
	}
	dst = t.AppendFormat(dst, shortTimeFormat)
	buf := bytes.NewBuffer(dst)
	fmt.Fprintf(buf, " %-18s| ", prefix)
	return buf.Bytes()
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
	fmt.Fprintf(sb, "%-*s %-*s %*s\n",
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
