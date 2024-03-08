# WARNING
this tool will not work if pools have different erazure coding settings.

# Notes 
 - The lister can be paused and continued, but keep in mind it's a lexical listing so it will miss new objects that are uploaded during it's pause period. The -reset flag must be removed to continue listing after pausing.
 - The checker can be paused and resumed. To restart the checking from the first file just run -reset

# Building the docker contains
```bash
$ docker build --tag check .
```

# Running the list generator
## Help menu
```bash
Usage of list:
  -bucket string
        defines the bucket used for listing
  -concurrency int
        controls how many objects we want to list per call to minio (default 1000)
  -ignoreMultipart
        Ignore multipart objects. This is required if the minio release does not includes the GetObjectAttributes api
  -insecure
        skip insecure TLS validation
  -key string
        minio private key (default "minioadmin")
  -logN int
        tell this program to print a report every Nth object listed (default 10000)
  -minio string
        sets the minio endpoint (default "127.0.0.1:9000")
  -reset
        re-create the list from scratch
  -secret string
        minio user secret (default "minioadmin")
  -sqldb string
        sets the sql lite db file (default "data.db")
```

## Syntax
```bash
docker run -v [VOLUME_NAME]:/data -d check:latest /check list -bucket [BUCKET_NAME] -sqldb /data/[DATABASE_FILE] -concurrency 1000 -key [MINIO_ACCESS_KEY] -secret [MINIO_SECRET_KEY] -minio [MINIO_ENDPOINT] --insecure --ignoreMultipart -logN 50000 -reset
```

## Example
```bash
docker run -v checker_data:/data -d check:latest /check list -bucket prod_data -sqldb /data/objcets.db -concurrency 1000 -key minioadmin -secret minioadmin -minio https://127.0.0.1:9000 --insecure --ignoreMultipart -logN 50000 -reset
```

# Running the checker
```bash
Usage of check:
  -concurrency int
        controls how many objects we check at the same time. We will always query [concurrency*3] objects from sqllite at a time. (default 10)
  -ignoreMultipart
        Ignore multipart objects. This is required if the minio release does not includes the GetObjectAttributes api
  -insecure
        skip insecure TLS validation
  -key string
        minio private key (default "minioadmin")
  -logN int
        tell this program to print a report every Nth object listed (default 10000)
  -minio string
        sets the minio endpoint (default "127.0.0.1:9000")
  -reset
        reset the cursor to 0
  -secret string
        minio user secret (default "minioadmin")
  -shards int
        defines how many data shards each object part has (default 2)
  -sleep int
        sets a sleep timer in Milliseconds that is enforced after processing every object
  -sqldb string
        sets the sql lite db file (default "data.db")
```

## Syntax
```bash
docker run -v [VOLUME_NAME]:/data -d check:latest /check check -sqldb /data/[DATABASE_FILE] -concurrency 1000 -key [MINIO_ACCESS_KEY] -secret [MINIO_SECRET_KEY] -minio [MINIO_ENDPOINT] --insecure --ignoreMultipart -logN 50000 -reset -shards [NUMBER_OF_OBJECT_SHARDS_BASED_ON_ERAZURE_SETTINGS]
```

## Example
```bash
docker run -v checker_data:/data -d check:latest /check check -sqldb /data/objects.db -concurrency 1000 -key minioadmin -secret minioadmin -minio https://127.0.0.1:9000 --insecure --ignoreMultipart -logN 50000 -reset -shards 2

```


# How to find number of shards (TODO)
totalDrivesPerSet - standardSCParity == object data shard
```bash
$ mc admin info --json [ALIAS] | jq ".info.backend.standardSCParity"
$ mc admin info --json [ALIAS] | jq ".info.backend.totalDrivesPerSet"
```
