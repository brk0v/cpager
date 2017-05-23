# cpager
vmtouch per cgroup per file (on golang)

Using **pagemap** and **kpagecgroup**: https://www.kernel.org/doc/Documentation/vm/pagemap.txt

### Example ###

```
root@server:~# ./cpager ~/big_db_dir/
Warning:  Don't follow symlinks for ~/big_db_dir/logs. If you want then use "-f" flag

         Files: 47
   Directories: 1
Resident Pages: 10590695/14694992 40.4G/56.1G 72.1%

cgmem inode    percent       pages        path
          -      27.9%     4104297        not charged
        470      54.4%     7999124        /sys/fs/cgroup/memory/some/cgroup/with/hierarchy1
       3461       0.0%           8        /sys/fs/cgroup/memory/some/cgroup/with/hierarchy2
        584      12.9%     1901403        /sys/fs/cgroup/memory/some/cgroup/with/hierarchy3
        291       4.7%      690160        /sys/fs/cgroup/memory/
```

### Dependencies ###

```
go get github.com/spf13/pflag
```
