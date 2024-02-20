# Double mirror setup

# Initial synchronization mirror
```bash
$ mc mirror source target
```
The first mirror needs to be a global sync. 
This will ensure all objects are synced across sites.

## On interuption
The first mirror run will need to be restart if it is interrupted.
Otherwise there is no way to ensure all objects have been copied.

# Real-time mirror
```bash
$ mc mirror --watch --newer-than 15m source target
```
This mirror ensures all new objects are replicated between source and target. 

## On interuption
Simply restart the process. 
If the process is not restarted within 15 minutes the --newer-than flag will need adjusting.
```bash
$ mc mirror --watch --newer-than [minutes_since_exiting]m source target
```
