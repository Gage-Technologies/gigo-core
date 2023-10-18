/*
 *
 *  *  *********************************************************************************
 *  *   GAGE TECHNOLOGIES CONFIDENTIAL
 *  *   __________________
 *  *
 *  *    Gage Technologies
 *  *    Copyright (c) 2021
 *  *    All Rights Reserved.
 *  *
 *  *   NOTICE:  All information contained herein is, and remains
 *  *   the property of Gage Technologies and its suppliers,
 *  *   if any.  The intellectual and technical concepts contained
 *  *   herein are proprietary to Gage Technologies
 *  *   and its suppliers and may be covered by U.S. and Foreign Patents,
 *  *   patents in process, and are protected by trade secret or copyright law.
 *  *   Dissemination of this information or reproduction of this material
 *  *   is strictly forbidden unless prior written permission is obtained
 *  *   from Gage Technologies.
 *
 *
 */

package utils

import (
	"time"
)

// Determine Holiday for VSCode Themes

func DetermineHoliday() int {
	today := time.Now()
	var holiday int

	easterMonth, easterDay := getEaster()

	// Halloween
	if today.Month() == 10 {
		holiday = 1
		return holiday
	}

	// Christmas
	if today.Month() == 12 && today.Day() <= 25 {
		holiday = 2
		return holiday
	}

	//New Years
	if (today.Month() == 12 && today.Day() >= 26) && (today.Month() == 1 && today.Day() <= 2) {
		holiday = 3
		return holiday
	}

	//Valentine's
	if today.Month() == 2 && today.Day() <= 14 {
		holiday = 4
		return holiday
	}

	//Easter
	if today.Month() == time.Month(easterMonth) && today.Day() >= easterDay {
		holiday = 5
		return holiday
	}

	//Independence
	if (today.Month() == 7 && today.Day() < 5) || (today.Month() == 6 && today.Day() <= 30) {
		holiday = 6
		return holiday
	}

	return 0
}

func getEaster() (int, int) {
	currentYear := time.Now().Year()
	// for type (by inference) and value assignment use :=
	// shorthand for   var month int = 3
	month := 3
	// determine the Golden number
	golden := (currentYear % 19) + 1
	// determine the century number
	century := currentYear/100 + 1
	// correct for the years that are not leap years
	xx := (3*century)/4 - 12
	// moon correction
	yy := (8*century+5)/25 - 5
	// find Sunday
	zz := (5*currentYear)/4 - xx - 10
	// determine epact
	// age of moon on January 1st of that year
	// (follows a cycle of 19 years)
	ee := (11*golden + 20 + yy - xx) % 30
	if ee == 24 {
		ee += 1
	}
	if (ee == 25) && (golden > 11) {
		ee += 1
	}
	// get the full moon
	moon := 44 - ee
	if moon < 21 {
		moon += 30
	}
	// up to Sunday
	day := (moon + 7) - ((zz + moon) % 7)
	// possibly up a month in easter_date
	if day > 31 {
		day -= 31
		month = 4
	}
	return month, day
}
