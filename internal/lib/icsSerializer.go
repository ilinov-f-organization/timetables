package lib

import (
	"fmt"
	"time"
	"timetables/internal/api"

	ics "github.com/arran4/golang-ical"
)

const productId = "scheduler-3000"

func SerializeICS(lessons []api.Lesson, name string) ([]byte, error) {
	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodPublish)
	cal.SetName(name)
	cal.SetProductId(productId)

	for _, lesson := range lessons {
		timetableDateStart := lesson.Timetable.StartDate
		event := cal.AddEvent(lesson.Id.String())
		event.SetDtStampTime(time.Now())

		startTime := timetableDateStart.AddDate(0, 0, *lesson.Day-1)
		startTime = startTime.Add(time.Duration(*lesson.TimeStart) * time.Minute)

		endTime := timetableDateStart.AddDate(0, 0, *lesson.Day-1)
		endTime = endTime.Add(time.Duration(*lesson.TimeEnd) * time.Minute)

		freqInterval := 1

		switch *lesson.RepeatRule {
		case 2:
			startTime = startTime.AddDate(0, 0, 7)
			endTime = endTime.AddDate(0, 0, 7)
			freqInterval = 2
		case 1:
			freqInterval = 2
		}

		event.SetStartAt(startTime)
		event.SetEndAt(endTime)
		event.AddRrule(fmt.Sprintf("FREQ=WEEKLY;INTERVAL=%d;UNTIL=%s", freqInterval, lesson.Timetable.EndDate.Format("20060102T150405Z")))

		event.SetSummary(*lesson.Category + " " + *lesson.Subject.Name)

		description := ""
		location := ""
		for _, assignment := range *lesson.TeacherLocationAssignments {
			description += fmt.Sprintf("%s -> %s\n", *assignment.Location.Name, *assignment.Teacher.Name)
			location += *assignment.Location.Name + " "
		}

		description += fmt.Sprintf("\nГруппы:\n")
		for _, subgroup := range *lesson.Subgroups {
			description += fmt.Sprintf("%s\n", *subgroup.Name)
		}
		event.SetLocation(location)
		event.SetDescription(description)
	}
	return []byte(cal.Serialize()), nil
}
