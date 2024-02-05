# Object Read Check 

# Functionality
1. Checker takes in a file list from `input.json`
2. Checker merges `done.json` and `input.json`
    - file present in `done.json` will not be processed. 
3. Checker will pull `[CONCURRENCY]` number of objects out of a queue and sleep `[SLEEPTIMER]` number of millisecond
   before reading each object.
4. Checker reads only `1024 bytes` from each object and then closes the connections.
5. Each processed object is saved to an output file: `[timestamp.out.json]`
6. the output files can be renamed to `done.json` in order to prevent parsing duplicates.

# NOTES
1. Errors will be printed to the console and saved to the out file.
2. Checker will stop if it cannot write to output file.
3. Example `done.json` `input.json` and `out.json` can be seen in the current directory
4. The checker can handle versioned objects

# Reading errors
Errors will be added to the out.json file as `Parsed` `Error` and `ReadTime`
Example output:
```json

{
    ..... 
    "Parsed":true,
    "Error":"",
    "ReadTime":0,
}
```

# Creating an input.json 
```bash
$ mc ls -r [ALIAS] --json --no-color > input.json
```

# building
```bash
$ cd ./object-read-check
$ go build -o check .
```

# Running 
- ENDPOINT = node/load balancer endpoint
- ACCESS_KEY = service account access key
- SECRET_KEY = service account secret key
- CONCURRENCY = how many object reads we can do at a time
- SLEEPTIMER = time in milliseconds to sleep between object reads

```bash
$ ./check [ENDPOINT] [ACCESS_KEY] [SECRET_KEY] [CONCURRENCY] [SLEEPTIMER]
```


