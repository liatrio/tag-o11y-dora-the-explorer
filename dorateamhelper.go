package main

type DoraTeam struct {
	Level                 string
	MinutesBetweenDeploys int
}

//	 Performance level					Elite:
//		Deployment Frequency: 				On-demand (multiple deploys per day)
//		Change lead time: 					Less than one day
//		Change failure rate: 				5%
//	 	Failed deployment recovery time: 	Less than one hour
func NewEliteDoraTeam() *DoraTeam {
	return &DoraTeam{
		Level:                 "Elite",
		MinutesBetweenDeploys: 240, // 4 hours
	}
}

//	 Performance level					High:
//		Deployment Frequency: 				Between once per day and once per week
//		Change lead time: 					Between one day and one week
//		Change failure rate: 				10%
//	 	Failed deployment recovery time: 	Less than one day
func NewHighDoraTeam() *DoraTeam {
	return &DoraTeam{
		Level:                 "High",
		MinutesBetweenDeploys: 60,
	}
}

//	 Performance level					Medium:
//		Deployment Frequency: 				Between once per week and once per month
//		Change lead time: 					Between one week and one month
//		Change failure rate: 				15%
//	 	Failed deployment recovery time: 	Between one day and one week
func NewMediumDoraTeam() *DoraTeam {
	return &DoraTeam{
		Level:                 "Medium",
		MinutesBetweenDeploys: 1440,
	}
}

//	 Performance level					Low:
//		Deployment Frequency: 				Between once per week and once per month
//		Change lead time: 					Between one week and one month
//		Change failure rate: 				64%
//	 	Failed deployment recovery time: 	Between one month and six months
func NewLowDoraTeam() *DoraTeam {
	return &DoraTeam{
		Level:                 "Low",
		MinutesBetweenDeploys: 10080,
	}
}
