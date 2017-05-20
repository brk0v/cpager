# cpager
vmtouch per cgroup

### Example ###

```
# ./cpager ~/big_db_mmaped_file
Working with file:  ~/big_db_mmaped_file
total pages: 2137736
total mmaped pages: 2127345
total unmapped pages: 10391
cgroup /sys/fs/cgroup/memory/some/cgroup/with/hierarchy (inode: 292) charges: 2127345 pages (99%)
```
