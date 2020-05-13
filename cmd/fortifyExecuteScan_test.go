package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/stretchr/testify/assert"

	"github.com/piper-validation/fortify-client-go/models"
)

type fortifyMock struct {
	Successive bool
}

func (f *fortifyMock) GetProjectByName(name string, autoCreate bool, projectVersion string) (*models.Project, error) {
	return &models.Project{Name: &name}, nil
}
func (f *fortifyMock) GetProjectVersionDetailsByProjectIDAndVersionName(id int64, name string, autoCreate bool, projectName string) (*models.ProjectVersion, error) {
	return &models.ProjectVersion{ID: id, Name: &name, Project: &models.Project{Name: &projectName}}, nil
}
func (f *fortifyMock) GetProjectVersionAttributesByProjectVersionID(id int64) ([]*models.Attribute, error) {
	return []*models.Attribute{}, nil
}
func (f *fortifyMock) SetProjectVersionAttributesByProjectVersionID(id int64, attributes []*models.Attribute) ([]*models.Attribute, error) {
	return attributes, nil
}
func (f *fortifyMock) CreateProjectVersionIfNotExist(projectName, projectVersionName, description string) (*models.ProjectVersion, error) {
	return &models.ProjectVersion{ID: 4711, Name: &projectVersionName, Project: &models.Project{Name: &projectName}}, nil
}
func (f *fortifyMock) LookupOrCreateProjectVersionDetailsForPullRequest(projectID int64, masterProjectVersion *models.ProjectVersion, pullRequestName string) (*models.ProjectVersion, error) {
	return &models.ProjectVersion{ID: 4712, Name: &pullRequestName, Project: masterProjectVersion.Project}, nil
}
func (f *fortifyMock) CreateProjectVersion(version *models.ProjectVersion) (*models.ProjectVersion, error) {
	return version, nil
}

func (f *fortifyMock) ProjectVersionCopyFromPartial(sourceID, targetID int64) error {
	return nil
}
func (f *fortifyMock) ProjectVersionCopyCurrentState(sourceID, targetID int64) error {
	return nil
}
func (f *fortifyMock) ProjectVersionCopyPermissions(sourceID, targetID int64) error {
	return nil
}
func (f *fortifyMock) CommitProjectVersion(id int64) (*models.ProjectVersion, error) {
	name := "Committed"
	return &models.ProjectVersion{ID: id, Name: &name}, nil
}
func (f *fortifyMock) MergeProjectVersionStateOfPRIntoMaster(downloadEndpoint, uploadEndpoint string, masterProjectID, masterProjectVersionID int64, pullRequestName string) error {
	return nil
}
func (f *fortifyMock) GetArtifactsOfProjectVersion(id int64) ([]*models.Artifact, error) {
	if id == 4711 {
		return []*models.Artifact{&models.Artifact{Status: "PROCESSED", UploadDate: models.Iso8601MilliDateTime(time.Now().UTC())}}, nil
	}
	if id == 4712 {
		return []*models.Artifact{&models.Artifact{Status: "ERROR_PROCESSING", UploadDate: models.Iso8601MilliDateTime(time.Now().UTC())}}, nil
	}
	if id == 4713 {
		return []*models.Artifact{&models.Artifact{Status: "REQUIRE_AUTH", UploadDate: models.Iso8601MilliDateTime(time.Now().UTC())}}, nil
	}
	if id == 4714 {
		return []*models.Artifact{&models.Artifact{Status: "PROCESSING", UploadDate: models.Iso8601MilliDateTime(time.Now().UTC())}}, nil
	}
	if id == 4715 {
		return []*models.Artifact{&models.Artifact{Status: "PROCESSED", Embed: &models.EmbeddedScans{[]*models.Scan{&models.Scan{BuildLabel: "/commit/test"}}}, UploadDate: models.Iso8601MilliDateTime(time.Now().UTC())}}, nil
	}
	return []*models.Artifact{}, nil
}
func (f *fortifyMock) GetFilterSetOfProjectVersionByTitle(id int64, title string) (*models.FilterSet, error) {
	return &models.FilterSet{}, nil
}
func (f *fortifyMock) GetIssueFilterSelectorOfProjectVersionByName(id int64, names []string, options []string) (*models.IssueFilterSelectorSet, error) {
	return &models.IssueFilterSelectorSet{}, nil
}
func (f *fortifyMock) GetProjectIssuesByIDAndFilterSetGroupedBySelector(id int64, filter, filterSetGUID string, issueFilterSelectorSet *models.IssueFilterSelectorSet) ([]*models.ProjectVersionIssueGroup, error) {
	group := "Audit All"
	total := int32(15)
	audited := int32(12)
	group2 := "Corporate Security Requirements"
	total2 := int32(20)
	audited2 := int32(11)
	return []*models.ProjectVersionIssueGroup{
		&models.ProjectVersionIssueGroup{ID: &group, TotalCount: &total, AuditedCount: &audited},
		&models.ProjectVersionIssueGroup{ID: &group2, TotalCount: &total2, AuditedCount: &audited2},
	}, nil
}
func (f *fortifyMock) ReduceIssueFilterSelectorSet(issueFilterSelectorSet *models.IssueFilterSelectorSet, names []string, options []string) *models.IssueFilterSelectorSet {
	return issueFilterSelectorSet
}
func (f *fortifyMock) GetIssueStatisticsOfProjectVersion(id int64) ([]*models.IssueStatistics, error) {
	return []*models.IssueStatistics{}, nil
}
func (f *fortifyMock) GenerateQGateReport(projectID, projectVersionID, reportTemplateID int64, projectName, projectVersionName, reportFormat string) (*models.SavedReport, error) {
	if !f.Successive {
		f.Successive = true
		return &models.SavedReport{Status: "Processing"}, nil
	}
	f.Successive = false
	return &models.SavedReport{Status: "Complete"}, nil
}
func (f *fortifyMock) GetReportDetails(id int64) (*models.SavedReport, error) {
	return &models.SavedReport{Status: "Complete"}, nil
}
func (f *fortifyMock) UploadResultFile(endpoint, file string, projectVersionID int64) error {
	return nil
}
func (f *fortifyMock) DownloadReportFile(endpoint string, projectVersionID int64) ([]byte, error) {
	return []byte("abcd"), nil
}
func (f *fortifyMock) DownloadResultFile(endpoint string, projectVersionID int64) ([]byte, error) {
	return []byte("defg"), nil
}

type pullRequestServiceMock struct{}

func (prService pullRequestServiceMock) ListPullRequestsWithCommit(ctx context.Context, owner, repo, sha string, opts *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error) {
	if owner == "A" {
		result := 17
		return []*github.PullRequest{&github.PullRequest{Number: &result}}, &github.Response{}, nil
	} else if owner == "C" {
		return []*github.PullRequest{}, &github.Response{}, errors.New("Test error")
	}
	return []*github.PullRequest{}, &github.Response{}, nil
}

type execRunnerMock struct {
	numExecutions int
	current       *execution
	executions    []*execution
}

type execution struct {
	dirValue   string
	envValue   []string
	outWriter  io.Writer
	errWriter  io.Writer
	executable string
	parameters []string
}

func (er *execRunnerMock) newExecution() *execution {
	newExecution := &execution{}
	er.executions = append(er.executions, newExecution)
	return newExecution
}

func (er *execRunnerMock) currentExecution() *execution {
	if nil == er.current {
		er.numExecutions = 0
		er.current = er.newExecution()
	}
	return er.current
}

func (er *execRunnerMock) SetDir(d string) {
	er.currentExecution().dirValue = d
}

func (er *execRunnerMock) SetEnv(e []string) {
	er.currentExecution().envValue = e
}

func (er *execRunnerMock) Stdout(out io.Writer) {
	er.currentExecution().outWriter = out
}

func (er *execRunnerMock) Stderr(err io.Writer) {
	er.currentExecution().errWriter = err
}
func (er *execRunnerMock) RunExecutable(e string, p ...string) error {
	er.numExecutions++
	er.currentExecution().executable = e
	er.currentExecution().parameters = p
	classpathPip := "/usr/lib/python35.zip;/usr/lib/python3.5;/usr/lib/python3.5/plat-x86_64-linux-gnu;/usr/lib/python3.5/lib-dynload;/home/piper/.local/lib/python3.5/site-packages;/usr/local/lib/python3.5/dist-packages;/usr/lib/python3/dist-packages;./lib"
	classpathMaven := "some.jar;someother.jar"
	if e == "python2" {
		er.currentExecution().outWriter.Write([]byte(classpathPip))
	} else if e == "mvn" {
		ioutil.WriteFile(strings.ReplaceAll(p[2], "-Dmdep.outputFile=", ""), []byte(classpathMaven), 755)
	}
	er.current = er.newExecution()
	return nil
}

func TestAnalyseUnauditedIssues(t *testing.T) {
	config := fortifyExecuteScanOptions{MustAuditIssueGroups: "Audit All, Corporate Security Requirements"}
	ff := fortifyMock{}
	influx := fortifyExecuteScanInflux{}
	name := "test"
	projectVersion := models.ProjectVersion{ID: 4711, Name: &name}
	auditStatus := map[string]string{}
	selectorSet := models.IssueFilterSelectorSet{
		FilterBySet: []*models.IssueFilterSelector{
			&models.IssueFilterSelector{
				GUID:        "1",
				DisplayName: "Folder",
				EntityType:  "ET1",
				SelectorOptions: []*models.SelectorOption{
					&models.SelectorOption{
						GUID: "abcd",
					},
				},
			},
			&models.IssueFilterSelector{
				GUID:        "2",
				DisplayName: "Analysis",
				EntityType:  "ET2",
			},
		},
	}
	issues := analyseUnauditedIssues(config, &ff, &projectVersion, &models.FilterSet{}, &selectorSet, &influx, auditStatus)
	assert.Equal(t, 12, issues)
}

func TestTriggerFortifyScan(t *testing.T) {
	t.Run("maven", func(t *testing.T) {
		runner := execRunnerMock{}
		config := fortifyExecuteScanOptions{BuildTool: "maven", AutodetectClasspath: true, BuildDescriptorFile: "./pom.xml", Memory: "-Xmx4G -Xms2G"}
		triggerFortifyScan(config, &runner, "test", "testLabel")

		assert.Equal(t, 3, runner.numExecutions)

		assert.Equal(t, "mvn", runner.executions[0].executable)
		assert.Equal(t, []string{"--file", "./pom.xml", "-Dmdep.outputFile=cp.txt", "-DincludeScope=compile", "-Dorg.slf4j.simpleLogger.log.org.apache.maven.cli.transfer.Slf4jMavenTransferListener=warn", "--batch-mode", "dependency:build-classpath"}, runner.executions[0].parameters)

		assert.Equal(t, "sourceanalyzer", runner.executions[1].executable)
		assert.Equal(t, []string{"-verbose", "-64", "-b", "test", "-Xmx4G", "-Xms2G", "-cp", "some.jar;someother.jar", "**/*.xml", "**/*.html", "**/*.jsp", "**/*.js", "src/main/resources/**/*", "src/main/java/**/*"}, runner.executions[1].parameters)

		assert.Equal(t, "sourceanalyzer", runner.executions[2].executable)
		assert.Equal(t, []string{"-verbose", "-64", "-b", "test", "-scan", "-Xmx4G", "-Xms2G", "-build-label", "testLabel", "-logfile", "target/fortify-scan.log", "-f", "target/result.fpr"}, runner.executions[2].parameters)
	})

	t.Run("pip", func(t *testing.T) {
		runner := execRunnerMock{}
		config := fortifyExecuteScanOptions{BuildTool: "pip", PythonVersion: "python2", AutodetectClasspath: true, BuildDescriptorFile: "./setup.py", PythonRequirementsFile: "./requirements.txt", PythonInstallCommand: "pip2 install --user", Memory: "-Xmx4G -Xms2G"}
		triggerFortifyScan(config, &runner, "test", "testLabel")

		assert.Equal(t, 5, runner.numExecutions)

		assert.Equal(t, "python2", runner.executions[0].executable)
		assert.Equal(t, []string{"-c", "import sys;p=sys.path;p.remove('');print(';'.join(p))"}, runner.executions[0].parameters)

		assert.Equal(t, "pip2", runner.executions[1].executable)
		assert.Equal(t, []string{"install", "--user", "-r", "./requirements.txt", ""}, runner.executions[1].parameters)

		assert.Equal(t, "pip2", runner.executions[2].executable)
		assert.Equal(t, []string{"install", "--user"}, runner.executions[2].parameters)

		assert.Equal(t, "sourceanalyzer", runner.executions[3].executable)
		assert.Equal(t, []string{"-verbose", "-64", "-b", "test", "-Xmx4G", "-Xms2G", "-python-path", "/usr/lib/python35.zip;/usr/lib/python3.5;/usr/lib/python3.5/plat-x86_64-linux-gnu;/usr/lib/python3.5/lib-dynload;/home/piper/.local/lib/python3.5/site-packages;/usr/local/lib/python3.5/dist-packages;/usr/lib/python3/dist-packages;./lib", ""}, runner.executions[3].parameters)

		assert.Equal(t, "sourceanalyzer", runner.executions[4].executable)
		assert.Equal(t, []string{"-verbose", "-64", "-b", "test", "-scan", "-Xmx4G", "-Xms2G", "-build-label", "testLabel", "-logfile", "target/fortify-scan.log", "-f", "target/result.fpr"}, runner.executions[4].parameters)
	})
}

func TestGenerateAndDownloadQGateReport(t *testing.T) {
	ffMock := fortifyMock{Successive: false}
	config := fortifyExecuteScanOptions{ReportTemplateID: 18, ReportType: "PDF"}
	name := "test"
	projectVersion := models.ProjectVersion{ID: 4711, Name: &name}
	project := models.Project{ID: 815, Name: &name}
	projectVersion.Project = &project

	t.Run("success", func(t *testing.T) {
		data, err := generateAndDownloadQGateReport(config, &ffMock, &project, &projectVersion)
		assert.NoError(t, err)
		assert.Equal(t, []byte("abcd"), data)
	})
}

func TestVerifyScanResultsFinishedUploading(t *testing.T) {
	ffMock := fortifyMock{Successive: false}

	t.Run("error no recent upload detected", func(t *testing.T) {
		config := fortifyExecuteScanOptions{DeltaMinutes: -1}
		err := verifyScanResultsFinishedUploading(config, &ffMock, 4711, "", &models.FilterSet{}, 0)
		assert.Error(t, err)
		assert.Equal(t, "No recent upload detected on Project Version", err.Error())
	})

	config := fortifyExecuteScanOptions{DeltaMinutes: 20}
	t.Run("success", func(t *testing.T) {
		err := verifyScanResultsFinishedUploading(config, &ffMock, 4711, "", &models.FilterSet{}, 0)
		assert.NoError(t, err)
	})

	t.Run("error processing", func(t *testing.T) {
		err := verifyScanResultsFinishedUploading(config, &ffMock, 4712, "", &models.FilterSet{}, 0)
		assert.Error(t, err)
		assert.Equal(t, "There are artifacts that failed processing for Project Version 4712\n/html/ssc/index.jsp#!/version/4712/artifacts?filterSet=", err.Error())
	})

	t.Run("error required auth", func(t *testing.T) {
		err := verifyScanResultsFinishedUploading(config, &ffMock, 4713, "", &models.FilterSet{}, 0)
		assert.Error(t, err)
		assert.Equal(t, "There are artifacts that require manual approval for Project Version 4713\n/html/ssc/index.jsp#!/version/4713/artifacts?filterSet=", err.Error())
	})

	t.Run("error polling timeout", func(t *testing.T) {
		err := verifyScanResultsFinishedUploading(config, &ffMock, 4714, "", &models.FilterSet{}, 1)
		assert.Error(t, err)
		assert.Equal(t, "Terminating after 0 minutes since artifact for Project Version 4714 is still in status PROCESSING", err.Error())
	})

	t.Run("success build label", func(t *testing.T) {
		err := verifyScanResultsFinishedUploading(config, &ffMock, 4715, "/commit/test", &models.FilterSet{}, 0)
		assert.NoError(t, err)
	})

	t.Run("error no artifacts", func(t *testing.T) {
		err := verifyScanResultsFinishedUploading(config, &ffMock, 4716, "", &models.FilterSet{}, 0)
		assert.Error(t, err)
		assert.Equal(t, "No uploaded artifacts for assessment detected for project version with ID 4716", err.Error())
	})
}

func TestCalculateTimeDifferenceToLastUpload(t *testing.T) {
	diffSeconds := calculateTimeDifferenceToLastUpload(models.Iso8601MilliDateTime(time.Now().UTC()), 1234)

	assert.Equal(t, true, diffSeconds < 1)
}

func TestExecuteTemplatedCommand(t *testing.T) {
	runner := execRunnerMock{}
	template := []string{"{{.Executable}}", "-c", "{{.Param}}"}
	context := map[string]string{"Executable": "test.cmd", "Param": "abcd"}
	executeTemplatedCommand(&runner, template, context)

	assert.Equal(t, "test.cmd", runner.executions[0].executable)
	assert.Equal(t, []string{"-c", "abcd"}, runner.executions[0].parameters)
}

func TestDeterminePullRequestMerge(t *testing.T) {
	config := fortifyExecuteScanOptions{CommitMessage: "Merge pull request #2462 from branch f-test", PullRequestMessageRegex: `(?m).*Merge pull request #(\d+) from.*`, PullRequestMessageRegexGroup: 1}

	t.Run("success", func(t *testing.T) {
		match := determinePullRequestMerge(config)
		assert.Equal(t, "2462", match, "Expected different result")
	})

	t.Run("no match", func(t *testing.T) {
		config.CommitMessage = "Some test commit"
		match := determinePullRequestMerge(config)
		assert.Equal(t, "", match, "Expected different result")
	})
}

func TestDeterminePullRequestMergeGithub(t *testing.T) {
	prServiceMock := pullRequestServiceMock{}

	t.Run("success", func(t *testing.T) {
		match, err := determinePullRequestMergeGithub(nil, fortifyExecuteScanOptions{Owner: "A"}, prServiceMock)
		assert.NoError(t, err)
		assert.Equal(t, "17", match, "Expected different result")
	})

	t.Run("no match", func(t *testing.T) {
		match, err := determinePullRequestMergeGithub(nil, fortifyExecuteScanOptions{Owner: "B"}, prServiceMock)
		assert.NoError(t, err)
		assert.Equal(t, "", match, "Expected different result")
	})

	t.Run("error", func(t *testing.T) {
		match, err := determinePullRequestMergeGithub(nil, fortifyExecuteScanOptions{Owner: "C"}, prServiceMock)
		assert.Error(t, err)
		assert.Equal(t, "", match, "Expected different result")
	})
}

func TestTranslateProject(t *testing.T) {
	t.Run("python", func(t *testing.T) {
		execRunner := execRunnerMock{}
		config := fortifyExecuteScanOptions{BuildTool: "pip", Memory: "-Xmx4G", Translate: `[{"pythonPath":"./some/path","pythonIncludes":"./**/*","pythonExcludes":"./tests/**/*"}]`}
		translateProject(&config, &execRunner, "/commit/7267658798797", "")
		assert.Equal(t, "sourceanalyzer", execRunner.executions[0].executable, "Expected different executable")
		assert.Equal(t, []string{"-verbose", "-64", "-b", "/commit/7267658798797", "-Xmx4G", "-python-path", "./some/path", "-exclude", "./tests/**/*", "./**/*"}, execRunner.executions[0].parameters, "Expected different parameters")
	})

	t.Run("asp", func(t *testing.T) {
		execRunner := execRunnerMock{}
		config := fortifyExecuteScanOptions{BuildTool: "windows", Memory: "-Xmx6G", Translate: `[{"aspnetcore":"true","dotNetCoreVersion":"3.5","exclude":"./tests/**/*","libDirs":"tmp/","src":"./**/*"}]`}
		translateProject(&config, &execRunner, "/commit/7267658798797", "")
		assert.Equal(t, "sourceanalyzer", execRunner.executions[0].executable, "Expected different executable")
		assert.Equal(t, []string{"-verbose", "-64", "-b", "/commit/7267658798797", "-Xmx6G", "-aspnetcore", "-dotnet-core-version", "3.5", "-exclude", "./tests/**/*", "-libdirs", "tmp/", "./**/*"}, execRunner.executions[0].parameters, "Expected different parameters")
	})

	t.Run("java", func(t *testing.T) {
		execRunner := execRunnerMock{}
		config := fortifyExecuteScanOptions{BuildTool: "maven", Memory: "-Xmx2G", Translate: `[{"classpath":"./classes/*.jar","extdirs":"tmp/","jdk":"1.8.0-21","source":"1.8","sourcepath":"src/ext/","src":"./**/*"}]`}
		translateProject(&config, &execRunner, "/commit/7267658798797", "")
		assert.Equal(t, "sourceanalyzer", execRunner.executions[0].executable, "Expected different executable")
		assert.Equal(t, []string{"-verbose", "-64", "-b", "/commit/7267658798797", "-Xmx2G", "-cp", "./classes/*.jar", "-extdirs", "tmp/", "-source", "1.8", "-jdk", "1.8.0-21", "-sourcepath", "src/ext/", "./**/*"}, execRunner.executions[0].parameters, "Expected different parameters")
	})

	t.Run("auto classpath", func(t *testing.T) {
		execRunner := execRunnerMock{}
		config := fortifyExecuteScanOptions{BuildTool: "maven", Memory: "-Xmx2G", Translate: `[{"classpath":"./classes/*.jar", "extdirs":"tmp/","jdk":"1.8.0-21","source":"1.8","sourcepath":"src/ext/","src":"./**/*"}]`}
		translateProject(&config, &execRunner, "/commit/7267658798797", "./WEB-INF/lib/*.jar")
		assert.Equal(t, "sourceanalyzer", execRunner.executions[0].executable, "Expected different executable")
		assert.Equal(t, []string{"-verbose", "-64", "-b", "/commit/7267658798797", "-Xmx2G", "-cp", "./WEB-INF/lib/*.jar", "-extdirs", "tmp/", "-source", "1.8", "-jdk", "1.8.0-21", "-sourcepath", "src/ext/", "./**/*"}, execRunner.executions[0].parameters, "Expected different parameters")
	})
}

func TestScanProject(t *testing.T) {
	config := fortifyExecuteScanOptions{Memory: "-Xmx4G"}

	t.Run("normal", func(t *testing.T) {
		execRunner := execRunnerMock{}
		scanProject(&config, &execRunner, "/commit/7267658798797", "label")
		assert.Equal(t, "sourceanalyzer", execRunner.executions[0].executable, "Expected different executable")
		assert.Equal(t, []string{"-verbose", "-64", "-b", "/commit/7267658798797", "-scan", "-Xmx4G", "-build-label", "label", "-logfile", "target/fortify-scan.log", "-f", "target/result.fpr"}, execRunner.executions[0].parameters, "Expected different parameters")
	})

	t.Run("quick", func(t *testing.T) {
		execRunner := execRunnerMock{}
		config.QuickScan = true
		scanProject(&config, &execRunner, "/commit/7267658798797", "")
		assert.Equal(t, "sourceanalyzer", execRunner.executions[0].executable, "Expected different executable")
		assert.Equal(t, []string{"-verbose", "-64", "-b", "/commit/7267658798797", "-scan", "-Xmx4G", "-quick", "-logfile", "target/fortify-scan.log", "-f", "target/result.fpr"}, execRunner.executions[0].parameters, "Expected different parameters")
	})
}

func TestAutoresolveClasspath(t *testing.T) {
	t.Run("success pip", func(t *testing.T) {
		execRunner := execRunnerMock{}
		dir, err := ioutil.TempDir("", "classpath")
		assert.NoError(t, err, "Unexpected error detected")
		defer os.RemoveAll(dir)
		file := filepath.Join(dir, "cp.txt")

		result := autoresolvePipClasspath("python2", []string{"-c", "import sys;p=sys.path;p.remove('');print(';'.join(p))"}, file, &execRunner)
		assert.Equal(t, "python2", execRunner.executions[0].executable, "Expected different executable")
		assert.Equal(t, []string{"-c", "import sys;p=sys.path;p.remove('');print(';'.join(p))"}, execRunner.executions[0].parameters, "Expected different parameters")
		assert.Equal(t, "/usr/lib/python35.zip;/usr/lib/python3.5;/usr/lib/python3.5/plat-x86_64-linux-gnu;/usr/lib/python3.5/lib-dynload;/home/piper/.local/lib/python3.5/site-packages;/usr/local/lib/python3.5/dist-packages;/usr/lib/python3/dist-packages;./lib", result, "Expected different result")
	})

	t.Run("success maven", func(t *testing.T) {
		execRunner := execRunnerMock{}
		dir, err := ioutil.TempDir("", "classpath")
		assert.NoError(t, err, "Unexpected error detected")
		defer os.RemoveAll(dir)
		file := filepath.Join(dir, "cp.txt")

		result := autoresolveMavenClasspath("pom.xml", file, &execRunner)
		assert.Equal(t, "mvn", execRunner.executions[0].executable, "Expected different executable")
		assert.Equal(t, []string{"--file", "pom.xml", fmt.Sprintf("-Dmdep.outputFile=%v", file), "-DincludeScope=compile", "-Dorg.slf4j.simpleLogger.log.org.apache.maven.cli.transfer.Slf4jMavenTransferListener=warn", "--batch-mode", "dependency:build-classpath"}, execRunner.executions[0].parameters, "Expected different parameters")
		assert.Equal(t, "some.jar;someother.jar", result, "Expected different result")
	})
}
