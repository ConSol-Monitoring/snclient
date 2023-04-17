# Tresholds & Metrics

## Metrics and units by check

### check_drivesize, check_memory

- used
- free
- used_pct
- free_pct
___
- B, KB, MB, GB, TB
- %
___
Ex: used_pct < 15  
Note: used < 15% and used_pct < 15 both work and are equal. used_pct < 15% currently doesn't work

### check_service

- status
___
- stopped, dead, startpending, stoppending, running, started
___
Ex: status != running

### check_uptime

- uptime
___
- s
___
Ex: uptime < 180s

### check_wmi

Keys of the query return  
Ex: Select Version, Caption from win32_OperatingSystem -> Version, Caption
___
- any?
___
Ex: 'Version not like 10.0'

## Operators

- <, >, <=, >=, =, !=, is, not ('=' and 'is' as well as '!=' and 'not' can be used interchangeably)
- like, not like (self-explanatory, should be used for check_wmi query returns)