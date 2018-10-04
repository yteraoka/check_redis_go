check_redis_go
==============

Nagios plugin for check Redis

Usage
-----

```
Usage:
  check_redis_go [OPTIONS]

Application Options:
  -H, --host=     Server hostname or IP address (default: 127.0.0.1)
  -p, --port=     TCP Port (default: 6379)
  -t, --timeout=  Timeout in second (default: 1.0)
  -a, --password= Password
  -r, --role=     master or slave (default: master)
  -w, --warn=     Warning threshold memory used % (default: 90)
  -c, --crit=     Critical threshold memory used % (default: 95)
  -v, --version   Show version and exit

Help Options:
  -h, --help      Show this help message
```
