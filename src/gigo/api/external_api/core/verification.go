package core

import (
	"encoding/json"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"testing"
)

func TestTemplate(t *testing.T, dir string) (map[string]interface{}, error) {

	// Construct the terraform options with default retryable errors to handle the most common
	// retryable errors in terraform testing.
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		// Set the path to the Terraform code that will be tested.
		TerraformDir: dir,
		// find more terraform options here https://pkg.go.dev/github.com/gruntwork-io/terratest/modules/terraform@v0.41.3#Options
	})

	// Clean up resources with "terraform destroy" at the end of the test.
	defer terraform.Destroy(t, terraformOptions)

	// Run "terraform validate". Fail the test if there are any errors.
	_, err := terraform.InitAndValidateE(t, terraformOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to validate terraform file: %v", err)
	}

	// Run "terraform init" and "terraform apply". Fail the test if there are any errors.
	_, err = terraform.InitAndApplyE(t, terraformOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize and apply terraform file: %v", err)
	}

	// Run `terraform output` to get the values of output variables and check they have the expected values.
	output := terraform.Output(t, terraformOptions, "hello_world")
	assert.Equal(t, "Hello, World!", output)

	return map[string]interface{}{"message": "Template successfully verified", "template_dir": dir}, nil
}

func ValidateContainerConfig(t *testing.T, dir string) (map[string]interface{}, error) {

	// open json data returned from webhook
	jsonData, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to open specified file path: %v", err)
	}

	// read json data
	byte, err := io.ReadAll(jsonData)

	// create empty interface to hold json data
	res := make(map[string]json.RawMessage)

	// unmarshal
	err = json.Unmarshal(byte, &res)

	// Construct the terraform options with default retryable errors to handle the most common
	// retryable errors in terraform testing.
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		// Set the path to the Terraform code that will be tested.
		TerraformDir: dir,
	})

	// Clean up resources with "terraform destroy" at the end of the test.
	defer terraform.Destroy(t, terraformOptions)

	// Run "terraform init" and "terraform apply". Fail the test if there are any errors.
	terraform.InitAndApply(t, terraformOptions)

	// Run `terraform output` to get the values of output variables and check they have the expected values.
	output := terraform.Output(t, terraformOptions, "hello_world")
	assert.Equal(t, "Hello, World!", output)

	return map[string]interface{}{"message": "Template successfully verified"}, nil
}
