package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/appconfig"
	"github.com/aws/aws-sdk-go/service/appconfigdata"
)

func getApplicationID(svc *appconfig.AppConfig, appName string) (string, error) {
	result, err := svc.ListApplications(&appconfig.ListApplicationsInput{})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			return "", aerr
		}
		return "", err
	}

	var appID string
	for _, a := range result.Items {
		if *a.Name == appName {
			appID = *a.Id
		}
	}
	return appID, nil
}

func getEnvironmentID(svc *appconfig.AppConfig, appID, envName string) (string, error) {
	envResult, err := svc.ListEnvironments(&appconfig.ListEnvironmentsInput{
		ApplicationId: aws.String(appID),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			return "", aerr
		}
		return "", err
	}

	var envID string
	for _, e := range envResult.Items {
		if *e.Name == envName {
			envID = *e.Id
		}
	}
	return envID, nil
}

func getConfigProfileID(svc *appconfig.AppConfig, appID, cpName string) (string, error) {
	confProfResult, err := svc.ListConfigurationProfiles(&appconfig.ListConfigurationProfilesInput{
		ApplicationId: aws.String(appID),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			return "", aerr
		}
		return "", err
	}

	var cpID string
	for _, cp := range confProfResult.Items {
		if *cp.Name == cpName {
			cpID = *cp.Id
		}
	}
	return cpID, nil
}

func getAppconfigDataToken(svc *appconfigdata.AppConfigData, appID, envID, cpID string) (string, error) {
	result, err := svc.StartConfigurationSession(&appconfigdata.StartConfigurationSessionInput{
		ApplicationIdentifier:          &appID,
		EnvironmentIdentifier:          &envID,
		ConfigurationProfileIdentifier: &cpID,
	})
	if err != nil {
		return "", err
	}

	return *result.InitialConfigurationToken, nil
}

func switchAccount(sess *session.Session, rolearn string) (*session.Session, aws.Config, error) {
	sCreds := stscreds.NewCredentials(sess, rolearn)
	sConfig := aws.Config{Region: sess.Config.Region, Credentials: sCreds}
	sSess, err := session.NewSession(&sConfig)
	return sSess, sConfig, err
}

func main() {
	var (
		awsAccessKeyID     string
		awsSecretAccessKey string
		region             string
		application        string
		environment        string
		configProfile      string
		switchRole         string
	)

	flag.StringVar(&awsAccessKeyID, "access_key_id", os.Getenv("AWS_ACCESS_KEY_ID"), "aws-access-key-id")
	flag.StringVar(&awsSecretAccessKey, "secret_access_key", os.Getenv("AWS_SECRET_ACCESS_KEY"), "aws-secret-access-key")
	flag.StringVar(&region, "region", os.Getenv("AWS_REGION"), "aws-region")
	flag.StringVar(&application, "app", "", "AppConfig application name")
	flag.StringVar(&environment, "env", "", "AppConfig environment name")
	flag.StringVar(&configProfile, "conf_profile", "", "Appconfig config-profile name")
	flag.StringVar(&switchRole, "switch_role", "", "switch account role ARN")

	flag.Parse()

	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(awsAccessKeyID, awsSecretAccessKey, ""),
	}))
	var config aws.Config
	var err error

	if len(switchRole) > 0 {
		sess, config, err = switchAccount(sess, switchRole)
		if err != nil {
			log.Fatalf("err: %v", err)
		}
	}

	svc := appconfig.New(sess, &config)

	appID, err := getApplicationID(svc, application)
	if err != nil {
		log.Fatalf("err: %v", err)
	}

	fmt.Printf("appID: %s\n", appID)

	envID, err := getEnvironmentID(svc, appID, environment)
	if err != nil {
		log.Fatalf("err: %v", err)
	}
	fmt.Printf("envID: %s\n", envID)

	cpID, err := getConfigProfileID(svc, appID, configProfile)
	if err != nil {
		log.Fatalf("err: %v", err)
	}

	fmt.Printf("config-profile-id: '%s'\n", cpID)

	dataSvc := appconfigdata.New(sess, &config)
	token, err := getAppconfigDataToken(dataSvc, appID, envID, cpID)

	if err != nil {
		log.Fatalf("err: %v", err)
	}

	for {
		time.Sleep(1 * time.Second)
		result, err := dataSvc.GetLatestConfiguration(&appconfigdata.GetLatestConfigurationInput{
			ConfigurationToken: aws.String(token),
		})
		if err != nil {
			log.Fatalf("err: %v", err)
		}

		fmt.Printf("config: '%s'\n", string(result.Configuration))
	}
}
