# Disk lister
This program will list anything mounted at /base or a path given in LISTER_BASE_DIR. 

# WARNING
There is no throttling in place.

# Building binary
```bash
$ go build -o lister .
```

# Run binary
```bash
export LISTER_BASE_PATH="./"
$ ./lister
```

# Building image
```bash
$ docker build --tag lister .
```

# Running container
Mount the path/drive you want to list at /base
```bash
$ docker run -d -v [LOCAL_PATH]:/base lister
```
