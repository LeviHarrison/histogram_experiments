package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	addr          = flag.String("listen-address", ":8080", "address to listen on for HTTP requests")
	dataset       = flag.String("dataset", "", "input file to read datasat from")
	resolution    = flag.Uint("resolution", 20, "sparse buckets resolution per power of 10, must be ≤255")
	zeroThreshold = flag.Float64("zero-threshold", 1e-128, "width of the “zero” bucket")
	timeFactor    = flag.Float64("time-factor", 0, "how fast to run the time simulation, 0 results in ingesting all observations as fast as possible")
)

func observe(in io.Reader) {
	var (
		his = promauto.NewHistogram(prometheus.HistogramOpts{
			Name:                       "histogram_experiment",
			Help:                       "Test histogram for an experiment.",
			SparseBucketsResolution:    uint8(*resolution),
			SparseBucketsZeroThreshold: *zeroThreshold,
		})
		s              = bufio.NewScanner(in)
		count          = 0
		start          = time.Now()
		simulatedStart time.Time
	)

	for s.Scan() {
		count++
		ss := strings.Split(s.Text(), " ")
		if len(ss) != 2 {
			log.Fatalln("Unexpected number of tokens in line", count, ":", len(ss))
		}
		ts, err := time.Parse(time.RFC3339Nano, ss[0])
		if err != nil {
			log.Fatalln("Cound not parse time stamp in line", count, ":", err)
		}
		if simulatedStart.IsZero() {
			simulatedStart = ts
		}
		if *timeFactor > 0 {
			desiredTimeOffset := time.Duration(float64(ts.Sub(simulatedStart)) / *timeFactor)
			currentTimeOffset := time.Since(start)
			time.Sleep(desiredTimeOffset - currentTimeOffset)
		}
		if duration, err := time.ParseDuration(ss[1]); err == nil {
			his.Observe(duration.Seconds())
		} else {
			// It doesn't appear to be a duration. Try raw number.
			v, err := strconv.ParseFloat(ss[1], 64)
			if err != nil {
				log.Fatalln("Could not parse value in line", count, ":", err)
			}
			his.Observe(v)
		}
	}
	if s.Err() != nil {
		log.Fatalln("Could not complete reading dataset file:", s.Err())
	}
	log.Println("Performed", count, "observations in", time.Since(start), ".")

}

func main() {
	flag.Parse()
	if *resolution > 255 {
		log.Fatalln("--resolution greater 255 not allowed, provided value:", *resolution)
	}

	http.Handle("/metrics", promhttp.Handler())

	f, err := os.Open(*dataset)
	if err != nil {
		log.Fatalln("Could not open dataset file:", err)
	}
	defer f.Close()

	go observe(f)

	log.Println("Serving metrics, SIGTERM to abort…")
	http.ListenAndServe(*addr, nil)
}
