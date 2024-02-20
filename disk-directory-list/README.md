# Disk lister
This program will list anything mounted at /base or a path given in LISTER_BASE_DIR. 

# Output
This program prints the directory structure to stdout. The results can be seen in container logs if used with
Docker/Kubernetes.

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
Mount a target at /base
```bash
$ docker run -d -v [target]:/base lister
```

