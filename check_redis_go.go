package main

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	flags "github.com/jessevdk/go-flags"
	"os"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	Host     string  `short:"H" long:"host"       description:"Server hostname or IP address" default:"127.0.0.1"`
	Port     int     `short:"p" long:"port"       description:"TCP Port" default:"6379"`
	Timeout  float64 `short:"t" long:"timeout"    description:"Timeout in second" default:"1.0"`
	Password string  `short:"a" long:"password"   description:"Password"`
	Role     string  `short:"r" long:"role"       description:"master or slave" default:"master"`
	Warn     float64 `short:"w" long:"warn"       description:"Warning threshold memory used %" default:"90"`
	Crit     float64 `short:"c" long:"crit"       description:"Critical threshold memory used %" default:"95"`
	Version  bool    `short:"v" long:"version"    description:"Show version and exit"`
}

const Version = "0.1"

const (
	NagiosOk       = 0
	NagiosWarning  = 1
	NagiosCritical = 2
	NagiosUnknown  = 3
)

type RedisConfig struct {
	MaxMemory int64 `redis:"maxmemory"`
}

func nagios_result(nagios_status int, message string) {
	var status_text string
	if nagios_status == NagiosOk {
		status_text = "OK"
	} else if nagios_status == NagiosWarning {
		status_text = "WARNING"
	} else if nagios_status == NagiosCritical {
		status_text = "CRITICAL"
	} else if nagios_status == NagiosUnknown {
		status_text = "UNKNOWN"
	}
	fmt.Printf("REDIS %s - %s\n", status_text, message)
	os.Exit(nagios_status)
}

func main() {
	var nagios_status int = NagiosOk
	var result_message, stats string

	var opts Options
	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(NagiosUnknown)
	}

	if opts.Version {
		fmt.Printf("check_redis_go version %s\n", Version)
		os.Exit(0)
	}

	if opts.Role != "" && opts.Role != "master" && opts.Role != "slave" {
		nagios_result(NagiosUnknown, fmt.Sprintf("Unknown role: %s", opts.Role))
	}

	server := opts.Host + ":" + strconv.Itoa(opts.Port)

	var dialopts []redis.DialOption

	dialopts = append(dialopts, redis.DialConnectTimeout(time.Duration(opts.Timeout)*time.Second))
	dialopts = append(dialopts, redis.DialReadTimeout(time.Duration(opts.Timeout)*time.Second))
	dialopts = append(dialopts, redis.DialWriteTimeout(time.Duration(opts.Timeout)*time.Second))

	if opts.Password != "" {
		dialopts = append(dialopts, redis.DialPassword(opts.Password))
	}

	c, err := redis.Dial("tcp", server, dialopts...)
	if err != nil {
		nagios_result(NagiosCritical, fmt.Sprintf("%s", err))
	}
	defer c.Close()

	// Measure PING/PONG response time
	t1 := time.Now()
	_, err = redis.String(c.Do("PING"))
	if err != nil {
		nagios_result(NagiosCritical, fmt.Sprintf("%s", err))
	}
	t2 := time.Now()
	ping_response_time := t2.Sub(t1)

	// check stats
	info, err := redis.String(c.Do("INFO"))
	if err != nil {
		nagios_result(NagiosCritical, fmt.Sprintf("%s", err))
	}

	var used_memory, maxmemory, total_system_memory int64
	var role, master_link_status string
	var percent_used float64

	for _, line := range strings.Split(info, "\r\n") {
		data := strings.SplitN(line, ":", 2)
		if data[0] == "role" {
			role = data[1]
		} else if data[0] == "used_memory" {
			used_memory, _ = strconv.ParseInt(data[1], 10, 64)
		} else if data[0] == "total_system_memory" {
			total_system_memory, _ = strconv.ParseInt(data[1], 10, 64)
		} else if data[0] == "master_link_status" {
			master_link_status = data[1]
		}
	}

	values, err := redis.Values(c.Do("CONFIG", "GET", "maxmemory"))
	if err != nil {
		nagios_result(NagiosCritical, fmt.Sprintf("%s", err))
	}

	var config RedisConfig

	err = redis.ScanStruct(values, &config)
	if err != nil {
		nagios_result(NagiosCritical, fmt.Sprintf("%s", err))
	}
	maxmemory = config.MaxMemory
	if maxmemory == 0 && total_system_memory != 0 {
		maxmemory = total_system_memory
	}

	if maxmemory > 0 {
		percent_used = float64(used_memory) / float64(maxmemory) * 100
		if percent_used >= opts.Crit {
			nagios_status = NagiosCritical
			result_message = fmt.Sprintf("Critical threshold (%.2f%%) exceeded", opts.Crit)
		} else if percent_used >= opts.Warn {
			nagios_status = NagiosWarning
			result_message = fmt.Sprintf("Warning threshold (%.2f%%) exceeded", opts.Warn)
		}
	} else {
		percent_used = 0
	}

	if opts.Role != "" {
		if role != opts.Role {
			nagios_status = NagiosCritical
			result_message = fmt.Sprintf("Unexpected role. Expected=%s, Actual=%s", opts.Role, role)
		} else if opts.Role == "slave" {
			if master_link_status != "up" {
				nagios_status = NagiosCritical
				result_message = fmt.Sprintf("master_link_status is not up (actual: %s)", master_link_status)
			}
		}
	}

	stats = fmt.Sprintf("Memory used %d/%d MiB (%.2f%%)", used_memory/1024/1024, maxmemory/1024/1024, percent_used)

	perf_str := fmt.Sprintf("|time=%.6fs;;;%.6f;%.6f", ping_response_time.Seconds(),
		0.0, opts.Timeout)

	nagios_result(nagios_status, stats+" "+result_message+perf_str)
}
