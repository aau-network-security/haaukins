# Troubleshooting

In this guideline, the way of handling troubles and possible reasons of troubles will be explained with some solutions. 

*The issues that you may face during active usage of Haaukins*

- [No space left on device](#no-space-left-on-device)
- (TODO) [Continuous exiting from environment](#continuous-exiting-from-environment)
-[Failed to create the VirtualBox object!](#vm-import-failed)
- (TODO) [Pool overlaps with other one on this address space](#todo)
*Issues which can be seeing in setting up development environment*

- (TODO) [Certificate Issue](#certificate-issue)
- (TODO) [Unable to create database client](#unable-to-create-database-client)
- (TODO) [Config file not found](#config-file-not-found)

## No space left on device

This error could be caused due to various reasons which includes redundant inodes, bad blocks in your volume or an application which fills your `/` root directory. 
However, if you are facing this error after active usage of Haaukins platform you may need to configure your docker volumes path, the content of docker volumes should not be written to `/` root path. 
Check [this guide](https://mrturkmen.com/no-space-left-on-device/) for solving `No space left on device ` error and see whether it is caused due to docker or not. 

There is another reason that you may face with this error when you are using Haaukins. In the project, we are using `ioutil.Tempfile` which is under the hood connected to `os.Tempfile` and if you do not specify first parameter for that function,
it will use the value of `os.TempDir` as first parameter and it will check and return following values. 

Definition of how [os.TempDir](https://golang.org/src/os/file.go?s=11019:11040#L348)` is finding out which directory to use for temporary files. 
> 	 TempDir returns the default directory to use for temporary files.
>    On Unix systems, it returns $TMPDIR if non-empty, else /tmp. On Windows,
>    it uses GetTempPath, returning the first non-empty value from %TMP%, %TEMP%, %USERPROFILE%,
>    or the Windows directory. On Plan 9, it returns /tmp.
> 	 The directory is neither guaranteed to exist nor have accessible permissions.

If your `$TMPDIR` not set and you are using linux, then Haaukins will use `/tmp` directory for writing any temporary files. 

You may need to set it to a place which is generally used for keeping data, e.g `/data/tmp` , rather than `/tmp` under root path. 

If any of them did not work for you [create issue](https://github.com/aau-network-security/haaukins/issues/new?assignees=&labels=&template=bug_report.md&title=)

Alternatively,symbolic links could be very useful for using a directory which is not under root

You can bound a symbolic link to `/tmp` as follows; 

```bash
ln -s /data/tmp /tmp 
```

Creating symbolic link means that the data will actually reside in `/data/tmp` however, the data can be accesible and usable from `/tmp`. 

## VM Import failed

This issue can be caused for different reasons, it could be not enough space on the folder where VM will be important or bad VM file (-bad ova file-), in this case some operations should be done in order to fix it. 

### Import VM failed due to no space

In our case, we have faced with import error which means that when we try to import VM, it failed due to not having space on the server. In this case, an inspection made the reason of failing very clear.  

All data regarding to VM and other applications stayed on `root`  path which is not a common case for managing servers. `root` path should contain essential programs and installations where operating system is requiring or an application which should be under a user' home folder. Therefore, `root` path should be as clean as possible, all related data to an application should be stayed under `data` path/folder. 

Example output of this error: 

```bash
10:02AM DBG getting path lock first_time=true path=/<user-daemon-path>/frontends/kali.ova
10:03AM ERR Error while creating new lab VBoxError [import /<user-daemon-path>/frontends/kali.ova --vsys 0 --vmname kali{ecc3ea71}]: 0%...10%...20%...30%...40%...50%...60%...70%...80%...90%...100%
Interpreting /<user-daemon-path>/frontends/kali.ova...
OK.
0%...10%...20%...30%...
Progress state: VBOX_E_FILE_ERROR
VBoxManage: error: Appliance import failed
VBoxManage: error: Could not create the imported medium '/<path-to-vms>/VirtualBox VMs/kali/kali-disk001_2.vmdk'.
VBoxManage: error: VMDK: cannot write allocated data block in '/<path-to-vms>/VirtualBox VMs/kali/kali-disk001_2.vmdk' (VERR_DISK_FULL)
VBoxManage: error: Details: code VBOX_E_FILE_ERROR (0x80bb0004), component ApplianceWrap, interface IAppliance
VBoxManage: error: Context: "RTEXITCODE handleImportAppliance(HandlerArg*)" at line 886 of file VBoxManageAppliance.cpp
Disks:
  vmdisk1	42949672960	-1	http://www.vmware.com/interfaces/specifications/vmdk.html#streamOptimized	kali-disk001.vmdk	-1	-1	
Virtual system 0:
 0: Suggested OS type: "Debian_64"
    (change with "--vsys 0 --ostype <type>"; use "list ostypes" to list all possible values)
 1: VM name specified with --vmname: "kali{ecc3ea71}"
 2: Number of CPUs: 2
    (change with "--vsys 0 --cpus <n>")
 3: Guest memory: 1024 MB
    (change with "--vsys 0 --memory <MB>")
 4: Sound card (appliance expects "", can change on import)
    (disable with "--vsys 0 --unit 4 --ignore")
 5: Network adapter: orig NAT, config 3, extra slot=0;type=NAT
 6: IDE controller, type PIIX4
    (disable with "--vsys 0 --unit 6 --ignore")
 7: IDE controller, type PIIX4
    (disable with "--vsys 0 --unit 7 --ignore")
 8: Hard disk image: source image=kali-disk001.vmdk, target path=/<path-to-vms>/VirtualBox VMs/kali/kali-disk001_2.vmdk, controller=6;channel=0
    (change target path with "--vsys 0 --unit 8 --disk path";
    disable with "--vsys 0 --unit 8 --ignore")
```


In order to overcome such a situation, it is always nice to have exclusive data folder for each users on the server where they reside their data regarding to their applications/research. 

#### Create data folder for each user on the system

In general, servers have data path which is much higher than normal `root` path. In order to make `root` path as clean as possible, all data under `/home/${USER}` should be migrated into `data` path 

Changing default home folder can help to overcome the problem. 


```bash 
$ sudo su 
$ mkdir /data/${USER}
$ chown -R ${USER}:${USER} /data/${USER}
$ usermod -d  /data/${USER} ${USER} # will change default home dir to /data/${USER}
$ mv /home/${USER}/* /data/${USER}
```

Following bash script could be used automated way of handling the operation. 


```
#!/bin/bash 

# do not include the user who logged in to server 
# make sure that user has admin privileges

declare -a users=("user1" "user2" "user3")

for user in "${users[@]}"
do
   mkdir /data/$user
   chown -R $user:$user /data/$user
   # or do whatever with individual element of the array
   usermod -d  /data/$user $user
   mv /home/$user/* /data/$user
done

``````

Do that operations for all users (-except the one who logged in to server-) who consumes a lot of places in terms of data. 

Afterwards, there will be no problems regarding to spaces until, `/data` path is full. 



### Import failed due to bad ova file 

It is very clear that the problem is directly related to corrupted ova file, in those cases update ova file with non-corrupted file. 

### Import failed due to no permission 

In some cases, permission is denied for importing VMs, in order to overcome, change the permission of the folder where VMs are generated with correct permissions. 

Permissions are changed through `chmod` command. 

Examples:

```bash 
$ chmod +rw /vms/
```
It will add to the permission of `/vms`  write and read permissions. 
