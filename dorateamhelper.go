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
		logger.Sugar().Infof("Last deploy was before %d minutes... skipping", d.MinutesBetweenDeployRange.LowerBound)
		return -1, nil
	}

	// If the last deployment was more than the upper bound of the DORA team's
	// deployment frequency, then we need to generate a deployment now.
	var minutesBetweenDeploys int
	if time.Since(lastDeploy) > time.Duration(d.MinutesBetweenDeployRange.UpperBound)*time.Minute {
		minutesBetweenDeploys = 1 // time.Ticker will panic if 0
	} else {
		// Generate a random number between the lower and upper bounds
		// of the DORA team's deployment frequency
		minutesBetweenDeploys = rand.Intn(
			d.MinutesBetweenDeployRange.UpperBound-d.MinutesBetweenDeployRange.LowerBound) +
			d.MinutesBetweenDeployRange.LowerBound
	}

	return minutesBetweenDeploys, nil
}
