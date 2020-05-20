# Troubleshooting

In this guideline, the way of handling troubles and possible reasons of troubles will be explained with some solutions. 

*The issues that you may face during active usage of Haaukins*

- [No space left on device](#no-space-left-on-device)
- (TODO) [Continuous exiting from environment](#continuous-exiting-from-environment)
- (TODO) [Failed to create the VirtualBox object!](#failed-to-import-vm)
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

