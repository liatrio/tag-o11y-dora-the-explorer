package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

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
			// Testing bounds
			// LowerBound: 1,
			// UpperBound: 2,
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
		MinutesBetweenDeployRange: Range{
			LowerBound: 1440,  // 24 hours
			UpperBound: 10080, // 7 days
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
		MinutesBetweenDeployRange: Range{
			LowerBound: 10080, // 1 week
			UpperBound: 40320, // 4 weeks
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
		MinutesBetweenDeployRange: Range{
			LowerBound: 40320,  // 4 weeks
			UpperBound: 201600, // 24 weeks
		},
	}
}

// Given a teams performance level this function will return the number of minutes
// until the next deployment should be generated. Takes into account when the
// last deployment was made and the range of minutes between deployments for the
// DORA team.
//
// Returns -1 if we should skip this deployment
func (d *DoraTeam) MinutesUntilNextDeployment(ctx context.Context, ghrc *GitHubRepoContext) (int, error) {
	recentDeployments, err := ghrc.GetLastDeployment(ctx)

	if err != nil {
		return 0, fmt.Errorf("Error getting latest deployments for %s/%s: %s", ghrc.org, ghrc.name, err)
	}

	lastDeploy := time.Unix(0, 0)
	if recentDeployments != nil {
		lastDeploy = recentDeployments.CreatedAt
	}

	// If the last deployment was less than the lower bound of the DORA team's
	// deployment frequency, then we don't need to generate a deployment.
	if time.Since(lastDeploy) < time.Duration(d.MinutesBetweenDeployRange.LowerBound)*time.Minute {
		return -1, nil
	}

	// If the last deployment was more than the upper bound of the DORA team's
	// deployment frequency, then we need to generate a deployment now.
	var minutesUntilNextDeploy int
	if time.Since(lastDeploy) > time.Duration(d.MinutesBetweenDeployRange.UpperBound)*time.Minute {
		minutesUntilNextDeploy = 1 // time.Ticker will panic if 0
	} else {
		// Generate a random number that is at most upper bound deploy range from the last deployment
		maxThresholdTime := lastDeploy.Add(time.Duration(d.MinutesBetweenDeployRange.UpperBound) * time.Minute)

		rangeOfMinutesUntilNextDeploy := int(time.Until(maxThresholdTime).Minutes())
		if rangeOfMinutesUntilNextDeploy <= 0 { // This case should not be possible but rand.Intn panics if 0
			rangeOfMinutesUntilNextDeploy = 1
		}

		minutesUntilNextDeploy = rand.Intn(rangeOfMinutesUntilNextDeploy)

		if minutesUntilNextDeploy == 0 {
			minutesUntilNextDeploy = 1
		}
	}

	return minutesUntilNextDeploy, nil
}
