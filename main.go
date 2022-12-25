package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/fatih/color"
	"github.com/spf13/viper"
)

type DeployConfig struct {
	AWSProfile string              `mapstructure:"awsprofile"`
	Stacks     []map[string]string `mapstructure:"stacks"`
}

func configLoad() (DeployConfig, error) {
	viper.AddConfigPath(".")
	viper.SetConfigName("conf")

	var deployConfig DeployConfig

	err := viper.ReadInConfig()
	if err != nil {
		return deployConfig, err
	}

	err = viper.Unmarshal(&deployConfig)

	return deployConfig, err
}

func stackExists(cfn *cloudformation.Client, stackName string) bool {
	params := cloudformation.ListStacksInput{}
	paginator := cloudformation.NewListStacksPaginator(cfn, &params)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			log.Fatal(err)
		}

		for _, stack := range page.StackSummaries {
			if stack.StackStatus == "DELETE_COMPLETE" {
				continue
			}

			if *stack.StackName == stackName {
				return true
			}
		}
	}

	return false
}

func parseTemplate(filename string) (string, error) {
	template, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	templateContent := string(template)

	return templateContent, nil
}

func parseParams() {

}

func createStack(cfn *cloudformation.Client, stackName string, template string) (*cloudformation.CreateStackOutput, error) {
	response, err := cfn.CreateStack(context.TODO(), &cloudformation.CreateStackInput{
		StackName:    &stackName,
		TemplateBody: &template,
		Capabilities: []types.Capability{"CAPABILITY_IAM", "CAPABILITY_NAMED_IAM"},
	})

	if err != nil {
		return &cloudformation.CreateStackOutput{}, err
	}

	waiter := cloudformation.NewStackCreateCompleteWaiter(cfn)

	err = waiter.Wait(context.TODO(), &cloudformation.DescribeStacksInput{}, 600)
	if err != nil {
		return &cloudformation.CreateStackOutput{}, err
	}

	return response, nil
}

func updateStack(cfn *cloudformation.Client, stackName string, template string) (*cloudformation.UpdateStackOutput, error) {
	response, err := cfn.UpdateStack(context.TODO(), &cloudformation.UpdateStackInput{
		StackName:    &stackName,
		TemplateBody: &template,
		Capabilities: []types.Capability{"CAPABILITY_IAM", "CAPABILITY_NAMED_IAM"},
	})
	if err != nil {
		return &cloudformation.UpdateStackOutput{}, err
	}

	waiter := cloudformation.NewStackUpdateCompleteWaiter(cfn)

	err = waiter.Wait(context.TODO(), &cloudformation.DescribeStacksInput{}, 600)
	if err != nil {
		return &cloudformation.UpdateStackOutput{}, err
	}

	return response, nil
}

func deploy(cfn *cloudformation.Client, deploycfg DeployConfig) {
	for _, stack := range deploycfg.Stacks {
		stackName := stack["name"]

		template, err := parseTemplate(stack["template"])
		if err != nil {
			log.Print(color.YellowString(err.Error()))
			continue
		}

		log.Printf("Checking if stack %s exists", stackName)

		if stackExists(cfn, stackName) {
			log.Printf("Stack %s exists, attempting to update.", stackName)
			if _, err := updateStack(cfn, stackName, template); err != nil {
				log.Printf(color.RedString("Deploy of stack %s failed"), stackName)
				log.Fatal(color.RedString(err.Error()))
			}
		} else {
			log.Printf("Stack %s does not exists, creating.", stackName)
			if _, err := createStack(cfn, stackName, template); err != nil {
				log.Printf(color.RedString("Deploy of stack %s failed"), stackName)
				log.Fatal(color.RedString(err.Error()))
			}
		}

		log.Printf(color.GreenString("Sucessfully deployed stack %s"), stackName)
	}
}

func main() {
	log.Print("Loading deployment configuration")
	deploycfg, err := configLoad()
	if err != nil {
		log.Fatal(color.RedString(err.Error()))
	}

	log.Print("Initializing aws environment")
	awscfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithSharedConfigProfile(deploycfg.AWSProfile),
	)

	if err != nil {
		log.Fatal(color.RedString(err.Error()))
	}

	cfn := cloudformation.NewFromConfig(awscfg)

	log.Print("Deploying")
	deploy(cfn, deploycfg)
}
