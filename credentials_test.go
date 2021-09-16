package awsbase

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/hashicorp/aws-sdk-go-base/awsmocks"
)

func TestAWSGetCredentials_static(t *testing.T) {
	testCases := []struct {
		Key, Secret, Token string
	}{
		{
			Key:    "test",
			Secret: "secret",
		}, {
			Key:    "test",
			Secret: "secret",
			Token:  "token",
		},
	}

	for _, testCase := range testCases {
		c := testCase

		cfg := Config{
			AccessKey: c.Key,
			SecretKey: c.Secret,
			Token:     c.Token,
		}

		creds, err := getCredentialsProvider(context.Background(), &cfg)
		if err != nil {
			t.Fatalf("unexpected '%[1]T' error getting credentials provider: %[1]s", err)
		}

		validateCredentialsProvider(creds, c.Key, c.Secret, c.Token, credentials.StaticCredentialsName, t)
		testCredentialsProviderWrappedWithCache(creds, t)
	}
}

// TestAWSGetCredentials_ec2Imds is designed to test the scenario of running Terraform
// from an EC2 instance, without environment variables or manually supplied
// credentials.
func TestAWSGetCredentials_ec2Imds(t *testing.T) {
	// clear AWS_* environment variables
	resetEnv := awsmocks.UnsetEnv(t)
	defer resetEnv()

	// capture the test server's close method, to call after the test returns
	ts := awsmocks.AwsMetadataApiMock(append(awsmocks.Ec2metadata_securityCredentialsEndpoints, awsmocks.Ec2metadata_instanceIdEndpoint, awsmocks.Ec2metadata_iamInfoEndpoint))
	defer ts()

	// An empty config, no key supplied
	cfg := Config{}

	creds, err := getCredentialsProvider(context.Background(), &cfg)
	if err != nil {
		t.Fatalf("unexpected '%[1]T' error getting credentials provider: %[1]s", err)
	}

	validateCredentialsProvider(creds, "Ec2MetadataAccessKey", "Ec2MetadataSecretKey", "Ec2MetadataSessionToken", ec2rolecreds.ProviderName, t)
	testCredentialsProviderWrappedWithCache(creds, t)

}

func TestAWSGetCredentials_configShouldOverrideEc2IMDS(t *testing.T) {
	resetEnv := awsmocks.UnsetEnv(t)
	defer resetEnv()
	// capture the test server's close method, to call after the test returns
	ts := awsmocks.AwsMetadataApiMock(append(awsmocks.Ec2metadata_securityCredentialsEndpoints, awsmocks.Ec2metadata_instanceIdEndpoint, awsmocks.Ec2metadata_iamInfoEndpoint))
	defer ts()
	testCases := []struct {
		Key, Secret, Token string
	}{
		{
			Key:    "test",
			Secret: "secret",
		}, {
			Key:    "test",
			Secret: "secret",
			Token:  "token",
		},
	}

	for _, testCase := range testCases {
		c := testCase

		cfg := Config{
			AccessKey: c.Key,
			SecretKey: c.Secret,
			Token:     c.Token,
		}

		creds, err := getCredentialsProvider(context.Background(), &cfg)
		if err != nil {
			t.Fatalf("unexpected '%[1]T' error: %[1]s", err)
		}

		validateCredentialsProvider(creds, c.Key, c.Secret, c.Token, credentials.StaticCredentialsName, t)
		testCredentialsProviderWrappedWithCache(creds, t)
	}
}

func TestAWSGetCredentials_shouldErrorWithInvalidEc2ImdsEndpoint(t *testing.T) {
	resetEnv := awsmocks.UnsetEnv(t)
	defer resetEnv()
	// capture the test server's close method, to call after the test returns
	ts := awsmocks.InvalidAwsEnv()
	defer ts()

	// An empty config, no key supplied
	cfg := Config{}

	_, err := getCredentialsProvider(context.Background(), &cfg)
	if err == nil {
		t.Fatal("expected error returned when getting creds w/ invalid EC2 IMDS endpoint")
	}
	if !IsNoValidCredentialSourcesError(err) {
		t.Fatalf("expected NoValidCredentialSourcesError, got '%[1]T': %[1]s", err)
	}

}

func TestAWSGetCredentials_sharedCredentialsFile(t *testing.T) {
	resetEnv := awsmocks.UnsetEnv(t)
	defer resetEnv()

	if err := os.Setenv("AWS_PROFILE", "myprofile"); err != nil {
		t.Fatalf("Error resetting env var AWS_PROFILE: %s", err)
	}

	fileEnvName := writeCredentialsFile(credentialsFileContentsEnv, t)
	defer os.Remove(fileEnvName)

	fileParamName := writeCredentialsFile(credentialsFileContentsParam, t)
	defer os.Remove(fileParamName)

	if err := os.Setenv("AWS_SHARED_CREDENTIALS_FILE", fileEnvName); err != nil {
		t.Fatalf("Error resetting env var AWS_SHARED_CREDENTIALS_FILE: %s", err)
	}

	// Confirm AWS_SHARED_CREDENTIALS_FILE is working
	credsEnv, err := getCredentialsProvider(context.Background(), &Config{
		Profile: "myprofile",
	})
	if err != nil {
		t.Fatalf("unexpected '%[1]T' error getting credentials provider from environment: %[1]s", err)
	}
	validateCredentialsProvider(credsEnv, "accesskey1", "secretkey1", "", sharedConfigCredentialsSource(fileEnvName), t)

	// Confirm CredsFilename overrides AWS_SHARED_CREDENTIALS_FILE
	credsParam, err := getCredentialsProvider(context.Background(), &Config{
		Profile:                "myprofile",
		SharedCredentialsFiles: []string{fileParamName},
	})
	if err != nil {
		t.Fatalf("unexpected '%[1]T' error getting credentials provider from configuration: %[1]s", err)
	}
	validateCredentialsProvider(credsParam, "accesskey2", "secretkey2", "", sharedConfigCredentialsSource(fileParamName), t)
}

var credentialsFileContentsEnv = `[myprofile]
aws_access_key_id = accesskey1
aws_secret_access_key = secretkey1
`

var credentialsFileContentsParam = `[myprofile]
aws_access_key_id = accesskey2
aws_secret_access_key = secretkey2
`

func writeCredentialsFile(credentialsFileContents string, t *testing.T) string {
	file, err := ioutil.TempFile(os.TempDir(), "terraform_aws_cred")
	if err != nil {
		t.Fatalf("Error writing temporary credentials file: %s", err)
	}
	_, err = file.WriteString(credentialsFileContents)
	if err != nil {
		t.Fatalf("Error writing temporary credentials to file: %s", err)
	}
	err = file.Close()
	if err != nil {
		t.Fatalf("Error closing temporary credentials file: %s", err)
	}
	return file.Name()
}

func validateCredentialsProvider(creds aws.CredentialsProvider, accesskey, secretkey, token, source string, t *testing.T) {
	v, err := creds.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("Error retrieving credentials: %s", err)
	}

	if v.AccessKeyID != accesskey {
		t.Errorf("AccessKeyID mismatch, expected: %q, got %q", accesskey, v.AccessKeyID)
	}
	if v.SecretAccessKey != secretkey {
		t.Errorf("SecretAccessKey mismatch, expected: %q, got %q", secretkey, v.SecretAccessKey)
	}
	if v.SessionToken != token {
		t.Errorf("SessionToken mismatch, expected: %q, got %q", token, v.SessionToken)
	}
	if v.Source != source {
		t.Errorf("Expected provider name to be %q, %q given", source, v.Source)
	}
}

func testCredentialsProviderWrappedWithCache(creds aws.CredentialsProvider, t *testing.T) {
	switch creds.(type) {
	case *aws.CredentialsCache:
		break
	default:
		t.Error("expected credentials provider to be wrapped with aws.CredentialsCache")
	}
}

func sharedConfigCredentialsSource(filename string) string {
	return fmt.Sprintf(sharedConfigCredentialsProvider+": %s", filename)
}
