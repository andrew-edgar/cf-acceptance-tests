package apps

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	. "github.com/cloudfoundry/cf-acceptance-tests/cats_suite_helpers"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/helpers"
	"github.com/cloudfoundry-incubator/cf-test-helpers/workflowhelpers"
	"github.com/cloudfoundry/cf-acceptance-tests/helpers/app_helpers"
	"github.com/cloudfoundry/cf-acceptance-tests/helpers/assets"
	"github.com/cloudfoundry/cf-acceptance-tests/helpers/logs"
	"github.com/cloudfoundry/cf-acceptance-tests/helpers/random_name"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

var _ = AppsDescribe("Environment Variables Groups", func() {

	var fetchEnvironmentVariables = func(groupType string) map[string]string {
		var session *Session
		workflowhelpers.AsUser(TestSetup.AdminUserContext(), Config.DefaultTimeoutDuration(), func() {
			session = cf.Cf("curl", fmt.Sprintf("/v2/config/environment_variable_groups/%s", groupType)).Wait()
			Expect(session).To(Exit(0))
		})

		var envMap map[string]string
		err := json.Unmarshal(session.Out.Contents(), &envMap)
		Expect(err).NotTo(HaveOccurred())

		return envMap
	}

	var marshalUpdatedEnv = func(envMap map[string]string) []byte {
		jsonObj, err := json.Marshal(envMap)
		Expect(err).NotTo(HaveOccurred())
		return jsonObj
	}

	var extendEnv = func(groupType, envVarName, envVarValue string) {
		envMap := fetchEnvironmentVariables(groupType)
		envMap[envVarName] = envVarValue
		jsonObj := marshalUpdatedEnv(envMap)

		command := fmt.Sprintf("set-%s-environment-variable-group", groupType)
		Expect(cf.Cf(command, string(jsonObj)).Wait()).To(Exit(0))
	}

	var revertExtendedEnv = func(groupType, envVarName string) {
		envMap := fetchEnvironmentVariables(groupType)
		delete(envMap, envVarName)
		jsonObj := marshalUpdatedEnv(envMap)

		apiUrl := fmt.Sprintf("/v2/config/environment_variable_groups/%s", groupType)
		Expect(cf.Cf("curl", apiUrl, "-X", "PUT", "-d", string(jsonObj)).Wait()).To(Exit(0))
	}

	Context("Staging environment variable groups", func() {
		var appName string
		var buildpackName string
		var envVarName string

		BeforeEach(func() {
			appName = random_name.CATSRandomName("APP")
			envVarName = fmt.Sprintf("CATS_STAGING_TEST_VAR_%s", strconv.Itoa(int(time.Now().UnixNano())))
		})

		AfterEach(func() {
			app_helpers.AppReport(appName)

			workflowhelpers.AsUser(TestSetup.AdminUserContext(), Config.DefaultTimeoutDuration(), func() {
				revertExtendedEnv("staging", envVarName)
			})

			Expect(cf.Cf("delete", appName, "-f", "-r").Wait(Config.CfPushTimeoutDuration())).To(Exit(0))
		})

		It("Applies environment variables while staging apps", func() {
			buildpackName = random_name.CATSRandomName("BPK")
			envVarValue := fmt.Sprintf("staging_env_value_%s", strconv.Itoa(int(time.Now().UnixNano())))

			workflowhelpers.AsUser(TestSetup.AdminUserContext(), Config.DefaultTimeoutDuration(), func() {
				extendEnv("staging", envVarName, envVarValue)
			})

			Expect(cf.Cf("push", appName, "-m", DEFAULT_MEMORY_LIMIT, "-b", buildpackName, "-p", assets.NewAssets().HelloWorld, "-d", Config.GetAppsDomain()).Wait(Config.CfPushTimeoutDuration())).To(Exit(1))

			Eventually(logs.Tail(Config.GetUseLogCache(), appName)).Should(Say(envVarValue))
		})
	})

	Context("Running environment variable groups", func() {
		var appName string
		var envVarName string

		BeforeEach(func() {
			appName = random_name.CATSRandomName("APP")
			envVarName = fmt.Sprintf("CATS_RUNNING_TEST_VAR_%s", strconv.Itoa(int(time.Now().UnixNano())))
		})

		AfterEach(func() {
			app_helpers.AppReport(appName)

			workflowhelpers.AsUser(TestSetup.AdminUserContext(), Config.DefaultTimeoutDuration(), func() {
				revertExtendedEnv("running", envVarName)
			})

			Expect(cf.Cf("delete", appName, "-f", "-r").Wait(Config.CfPushTimeoutDuration())).To(Exit(0))
		})

		It("Applies correct environment variables while running apps", func() {
			envVarValue := fmt.Sprintf("running_env_value_%s", strconv.Itoa(int(time.Now().UnixNano())))
			workflowhelpers.AsUser(TestSetup.AdminUserContext(), Config.DefaultTimeoutDuration(), func() {
				extendEnv("running", envVarName, envVarValue)
			})

			Expect(cf.Cf("push",
				appName,
				"-b", Config.GetBinaryBuildpackName(),
				"-m", DEFAULT_MEMORY_LIMIT,
				"-p", assets.NewAssets().Catnip,
				"-c", "./catnip",
				"-d", Config.GetAppsDomain()).Wait(Config.CfPushTimeoutDuration())).To(Exit(0))

			env := helpers.CurlApp(Config, appName, "/env.json")

			Expect(env).To(ContainSubstring(envVarValue))
		})
	})
})
