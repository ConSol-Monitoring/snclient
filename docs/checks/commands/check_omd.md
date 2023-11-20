---
title: omd
---

## check_omd

Check omd site status.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_omd
    OK - site demo: running |...

Check **specific** site by site filter:

    check_omd site=mode
    OK - site demo: running |...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name    check_nrpe
        command_line    $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name               testhost
        service_description     check_omd
        use                     generic-service
        check_command           check_nrpe!check_omd!
    }

## Argument Defaults

| Argument      | Default Value                                       |
| ------------- | --------------------------------------------------- |
| filter        | autostart = 1                                       |
| warning       | state == 1                                          |
| critcal       | state >= 2                                          |
| empty-state   | 3 (UNKNOWN)                                         |
| empty-syntax  | check_omd failed to find any site with this filter. |
| top-syntax    | \${status} - \${list}                               |
| ok-syntax     |                                                     |
| detail-syntax | site \${site}: \${status}\${failed_services_txt}    |

## Check Specific Arguments

| Argument | Description           |
| -------- | --------------------- |
| exclude  | Skip this omd service |
| site     | Show this site only   |

## Attributes

### Check Specific Attributes

these can be used in filters and thresholds (along with the default attributes):

| Attribute           | Description                                                                             |
| ------------------- | --------------------------------------------------------------------------------------- |
| site                | OMD site name                                                                           |
| autostart           | Configuration value of 'autostart': 0/1                                                 |
| state               | Return code of omd status, 0 - running, 1 - partially running, 2 - stopped, 3 - unknown |
| status              | Text status: (running, partially running, stopped, unknown)                             |
| failed_services     | List of failed services                                                                 |
| failed_services_txt | More usable form of failed services list                                                |
