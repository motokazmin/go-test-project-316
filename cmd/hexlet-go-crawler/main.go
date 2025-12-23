package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"code/crawler"
)

func main() {
	var (
		depth     = flag.Int("depth", 10, "crawl depth")
		retries   = flag.Int("retries", 1, "number of retries for failed requests")
		delayStr  = flag.String("delay", "0s", "delay between requests")
		timeout   = flag.Duration("timeout", 15*time.Second, "per-request timeout")
		rps       = flag.Int("rps", 0, "limit requests per second (overrides delay)")
		userAgent = flag.String("user-agent", "", "custom user agent")
		workers   = flag.Int("workers", 4, "number of concurrent workers")
		indent    = flag.Bool("indent", true, "indent JSON output")
		help      = flag.Bool("help", false, "show help")
		h         = flag.Bool("h", false, "show help")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `NAME:
   hexlet-go-crawler - analyze a website structure

USAGE:
   hexlet-go-crawler [global options] command [command options] <url>

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --depth value       crawl depth (default: 10)
   --retries value     number of retries for failed requests (default: 1)
   --delay value       delay between requests (example: 200ms, 1s) (default: 0s)
   --timeout value     per-request timeout (default: 15s)
   --rps value         limit requests per second (overrides delay) (default: 0)
   --user-agent value  custom user agent
   --workers value     number of concurrent workers (default: 4)
   --help, -h          show help
`)
	}

	flag.Parse()

	if *help || *h {
		flag.Usage()
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(0)
	}

	urlStr := args[0]

	// Парсим delay
	delay, err := time.ParseDuration(*delayStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid delay format: %v\n", err)
		os.Exit(0)
	}

	// Если rps установлен, переопределяем delay
	if *rps > 0 {
		delay = time.Second / time.Duration(*rps)
	}

	// Создаем опции
	opts := crawler.Options{
		URL:        urlStr,
		Depth:      *depth,
		Retries:    *retries,
		Delay:      delay,
		Timeout:    *timeout,
		UserAgent:  *userAgent,
		Workers:    *workers,
		IndentJSON: *indent,
		HTTPClient: &http.Client{},
	}

	// Запускаем анализ
	report, err := crawler.Analyze(context.Background(), opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(0)
	}

	// Выводим результат
	fmt.Println(string(report))
	os.Exit(0)
}

