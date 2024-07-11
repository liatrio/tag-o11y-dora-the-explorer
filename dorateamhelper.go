package main

type Range struct {
	LowerBound int
	UpperBound int
}

type DoraTeam struct {
	Level                     string
	MinutesBetweenDeployRange Range
}

//	 Performance level					Elite:
//		Deployment Frequency: 				On-demand (multiple deploys per day)
//		Change lead time: 					Less than one day
//		Change failure rate: 				5%
//	 	Failed deployment recovery time: 	Less than one hour
func NewEliteDoraTeam() *DoraTeam {
	return &DoraTeam{
		Level: "Elite",
		MinutesBetweenDeployRange: Range{ // Between 1 and 12 hours
			LowerBound: 60,
			UpperBound: 720,
		},
	}
}

//	 Performance level					High:
//		Deployment Frequency: 				Between once per day and once per week
//		Change lead time: 					Between one day and one week
//		Change failure rate: 				10%
//	 	Failed deployment recovery time: 	Less than one day
func NewHighDoraTeam() *DoraTeam {
	return &DoraTeam{
		Level: "High",
		MinutesBetweenDeployRange: Range{ // Between 1440 and 10080 minutes
			LowerBound: 1440,
			UpperBound: 10080,
		},
	}
}

//	 Performance level					Medium:
//		Deployment Frequency: 				Between once per week and once per month
//		Change lead time: 					Between one week and one month
//		Change failure rate: 				15%
//	 	Failed deployment recovery time: 	Between one day and one week
func NewMediumDoraTeam() *DoraTeam {
	return &DoraTeam{
		Level: "Medium",
		MinutesBetweenDeployRange: Range{ // Between 10080 and 40320 minutes
			LowerBound: 10080,
			UpperBound: 40320,
		},
	}
}

//	 Performance level					Low:
//		Deployment Frequency: 				Between once per week and once per month
//		Change lead time: 					Between one week and one month
//		Change failure rate: 				64%
//	 	Failed deployment recovery time: 	Between one month and six months
func NewLowDoraTeam() *DoraTeam {
	return &DoraTeam{
		Level: "Low",
		MinutesBetweenDeployRange: Range{ // Between 40320 and 201600 minutes
			LowerBound: 40320,
			UpperBound: 201600,
		},
	}
}
