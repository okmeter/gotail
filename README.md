# gotail
Go package for tailing file

### Installing 

Installing gotail may be done via the usual go get procedure:
```
go get gopkg.in/okmeter/gotail.v1
```

### Hot to use

```
config := tail.NewConfig()
t, err := tail.NewTail("/var/log/nginx/access.log", 0, config)
defer t.Close()
for {
	line, err := t.ReadLine()
	if err != nil {
	  // process error
	}
	// process line
}
```
