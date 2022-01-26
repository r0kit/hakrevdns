package main

import (
    "bufio"
    "context"
    "encoding/json"
    flags "github.com/jessevdk/go-flags"
    "fmt"
    "net"
    "sync"
    "strings"
    "os"
)

var opts struct {
        Threads int `short:"t" long:"threads" default:"8" description:"How many threads should be used"`
        ResolverIP string `short:"r" long:"resolver" description:"IP of the DNS resolver to use for lookups"`
        Protocol   string `short:"P" long:"protocol" choice:"tcp" choice:"udp" default:"udp" description:"Protocol to use for lookups"`
        Port       uint16 `short:"p" long:"port" default:"53" description:"Port to bother the specified DNS resolver on"`
	Domain     bool   `short:"d" long:"domain" description:"Output only domains"`
}

func main() {
        _, err := flags.ParseArgs(&opts, os.Args)
        if err != nil{
            os.Exit(1)
        }

        // default of 8 threads
        numWorkers := opts.Threads

        work := make(chan string)
        go func() {
            s := bufio.NewScanner(os.Stdin)
            for s.Scan() {
                work <- s.Text()
            }
            close(work)
        }()

        wg := &sync.WaitGroup{}

        for i := 0; i < numWorkers; i++ {
            wg.Add(1)
            go doWork(work, wg)
        }
        wg.Wait()
}


type IP struct {
    Address string
    RDNS    string
}

type Entry struct {
    Host    string
    IPs     []IP
}

func doWork(work chan string, wg *sync.WaitGroup) {
    defer wg.Done()
    var r *net.Resolver

    if opts.ResolverIP != "" {
            r = &net.Resolver{
                    PreferGo: true,
                    Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
                            d := net.Dialer{}
                            return d.DialContext(ctx, opts.Protocol, fmt.Sprintf("%s:%d", opts.ResolverIP, opts.Port))
                    },
            }
    }

    // Resolve hostname first
    for host := range work {
        entry := Entry {
            Host: host,
            IPs: make([]IP, 0),
        }

        ips, err := r.LookupIP(context.Background(), "ip4", host)
        if err != nil {
            // fmt.Fprintf(os.Stderr, "Could not get IPs: %v\n", err)
            continue
        }

        for _, ip := range ips {

            // Do revdns
            addr, err := r.LookupAddr(context.Background(), ip.String())
            if err != nil {
                    continue
            }

            var rdns string
            for _, a := range addr {
                    if opts.Domain {
                            rdns = strings.TrimRight(a, ".")
                    } else {
                            rdns = a
                    }
            }

            resolved := IP {
                Address: ip.String(),
                RDNS: rdns,
            }
            entry.IPs = append(entry.IPs, resolved)
        }


        result, _ := json.Marshal(entry)
        fmt.Printf("%s\n", string(result))
    }
}
