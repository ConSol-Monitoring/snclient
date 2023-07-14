---
title: check_files
---

# check_files

Check various aspects of a file and/or folder.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Metrics](#metrics)

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
|:-------:|:-----:|:-------:|:------:|
| :construction: | :construction: | :construction: | :construction: |

## Examples

### **Default check**

    check_files path=c:/windows warn=size>2MB max-depth=1
    WARNING: 1/28 files (warning(explorer.exe))

## Argument Defaults

| Argument | Default Value |
| --- | --- |
empty-state | 3 (Unknown) |
top-syntax | %(problem_count)/%(count) files (%(problem_list)) |
ok-syntax | All %(count) files are ok |
empty-syntax | No files found |
detail-syntax | %(name) |

### **Check specific arguments**

| Argument | Description |
| --- | --- |
| path | Path in which to search for files |
| file | Alias for path |
| paths | A comma seperated list of paths |
| pattern | Pattern of files to search for |
| max-depth | Maximum recursion depth |
| timezone | Sets the timezone for time metrics (default is local time) |

## Metrics

#### **Check specific metrics**

| Metric | Description |
| --- | --- |
| access | Last access time |
| age | Seconds since file was last written |
| creation | When file was created |
| file | Name of the file |
| filename | Name of the file |
| name | Name of the file |
| path | Path of the file |
| line_count | Number of lines in the files (text files) |
| size | File size|
| type | Type of item (file or dir)|
| written | When file was last written to |
| write | Alias for written |