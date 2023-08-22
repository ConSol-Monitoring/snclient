package snclient

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"pkg/utils"
)

type UpdateHours struct {
	min *int64
	max *int64
}

func NewUpdateHours(updateHours string) (res []UpdateHours, err error) {
	res = make([]UpdateHours, 0)
	token := utils.TokenizeBy(updateHours, ", \t\n\r", false, false)

	for _, def := range token {
		hours := strings.Split(def, "-")
		switch {
		case len(hours) == 1:
			num, err := strconv.ParseInt(hours[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("syntax error: %s", err.Error())
			}
			res = append(res, UpdateHours{min: &num})
		case len(hours) == 2:
			num1, err := strconv.ParseInt(hours[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("syntax error: %s", err.Error())
			}
			num2, err := strconv.ParseInt(hours[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("syntax error: %s", err.Error())
			}
			res = append(res, UpdateHours{min: &num1, max: &num2})
			if num1 >= num2 {
				return nil, fmt.Errorf("syntax error, hour2 must be bigger than hour2")
			}
		default:
			return nil, fmt.Errorf("syntax error, ex.: from-to")
		}
	}

	return res, nil
}

func (uh *UpdateHours) InTime(dt time.Time) bool {
	hourNum := int64(dt.Hour())
	switch {
	case uh.max == nil:
		if hourNum == *uh.min {
			return true
		}
	default:
		if hourNum >= *uh.min && hourNum < *uh.max {
			return true
		}
	}

	return false
}

type UpdateDays struct {
	min *time.Weekday
	max *time.Weekday
}

func NewUpdateDays(updateDays string) (res []UpdateDays, err error) {
	res = make([]UpdateDays, 0)
	token := utils.TokenizeBy(updateDays, ", \t\n\r", false, false)

	for _, def := range token {
		days := strings.Split(def, "-")
		switch {
		case len(days) == 1:
			udays := UpdateDays{}
			num, err := udays.day2num(days[0])
			if err != nil {
				return nil, err
			}
			udays.min = &num
			res = append(res, udays)
		case len(days) == 2:
			udays := UpdateDays{}
			num1, err := udays.day2num(days[0])
			if err != nil {
				return nil, err
			}
			udays.min = &num1
			num2, err := udays.day2num(days[1])
			if err != nil {
				return nil, err
			}
			udays.max = &num2
			if int64(num2) > 0 && int64(num1) >= int64(num2) {
				return nil, fmt.Errorf("syntax error, day2 must be bigger than after day1")
			}
			res = append(res, udays)
		default:
			return nil, fmt.Errorf("syntax error, ex.: from-to")
		}
	}

	return res, nil
}

func (ud *UpdateDays) InTime(dt time.Time) bool {
	weekday := dt.Weekday()
	switch {
	case ud.max == nil:
		if weekday == *ud.min {
			return true
		}
	default:
		if weekday >= *ud.min {
			if weekday <= *ud.max {
				return true
			}
			if *ud.max == time.Sunday {
				return true
			}
		}
	}

	return false
}

func (ud *UpdateDays) day2num(str string) (time.Weekday, error) {
	switch strings.ToLower(str) {
	case "mon":
		return time.Monday, nil
	case "tue":
		return time.Tuesday, nil
	case "wed":
		return time.Wednesday, nil
	case "thu":
		return time.Thursday, nil
	case "fri":
		return time.Friday, nil
	case "sat":
		return time.Saturday, nil
	case "sun":
		return time.Sunday, nil
	default:
		return 0, fmt.Errorf("unknown day: %s", str)
	}
}
