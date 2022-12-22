package test

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/iam/v1"
	"math/rand"
	"os"
	"path/filepath"
	policysupport "policy-conditions/policySupport"
	"policy-conditions/policySupport/providers/gcpBind"
	"runtime"
	"testing"
	"time"
)

var gcpMapper = gcpBind.New(map[string]string{})

func getIdqlFile() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(file, "../resources/data.json")
}

func TestProduceAndParseGcp(t *testing.T) {
	var err error
	policies, err := policysupport.ParsePolicyFile(getIdqlFile())
	assert.NoError(t, err, "File %s not parsed", getIdqlFile())

	bindAssignments := gcpMapper.MapPoliciesToBindings(policies)

	rand.Seed(time.Now().UnixNano())
	dir := t.TempDir()

	runId := rand.Uint64()

	// We will generate 3 output variants to test the parser

	bindingAssignFile := filepath.Join(dir, fmt.Sprintf("bindAssign-%d.json", runId))
	bindingsAssignFile := filepath.Join(dir, fmt.Sprintf("bindAssigns-%d.json", runId))
	bindingFile := filepath.Join(dir, fmt.Sprintf("binding-%d.json", runId))

	//Write a single binding
	assert.NoError(t, WriteObj(bindingFile, bindAssignments[0].Bindings[0]), "Single bind write")

	//Write out a single bind assignment
	assert.NoError(t, WriteObj(bindingAssignFile, bindAssignments[0]), "Single bind assignment write")

	//Write out all assignments
	assert.NoError(t, WriteObj(bindingsAssignFile, bindAssignments), "Single bind assignment write")

	// Parse a simple binding
	bindRead, err := gcpBind.ParseFile(bindingFile)
	assert.NoError(t, err, "Read a single binding")

	assert.Equal(t, 1, len(bindRead), "Check 1 GcpBindAssignment returned")
	resId := bindRead[0].ResourceId
	assert.Equal(t, "", resId)

	// Parse a single assignment
	bindAssign, err := gcpBind.ParseFile(bindingAssignFile)
	assert.NoError(t, err, "Read a single binding assignment")

	assert.Equal(t, 1, len(bindAssign), "Check 1 GcpBindAssignment returned")
	resId = bindAssign[0].ResourceId
	assert.NotEqual(t, "", resId)

	// Parse a multiple assignment
	bindAssigns, err := gcpBind.ParseFile(bindingsAssignFile)
	assert.NoError(t, err, "Read multiple binding assignments")

	assert.Equal(t, 3, len(bindAssigns), "Check 4 GcpBindAssignment returned")
	p1 := bindAssigns[0]
	p2 := bindAssigns[1]
	resId1 := p1.ResourceId
	resId2 := p2.ResourceId

	assert.NotEqual(t, resId1, resId2, "Check resource ids are different")

	copyPolcies, err := gcpMapper.MapBindingAssignmentsToPolicy(bindAssigns)
	assert.NoError(t, err, "Check error after mapping bindings back to policies")
	assert.Equal(t, 4, len(copyPolcies), "4 policies returned")
}

func TestReadGcp(t *testing.T) {
	// Read a single policy

}

func WriteObj(path string, data interface{}) error {
	var polBytes []byte
	switch pol := data.(type) {
	case iam.Binding:
		polBytes, err := json.MarshalIndent(pol, "", "  ")
		if err != nil {
			fmt.Println(err.Error())
		}
		//	fmt.Println(string(polBytes))
		return os.WriteFile(path, polBytes, 0644)

	case []*gcpBind.GcpBindAssignment, *gcpBind.GcpBindAssignment:
		polBytes, err := json.MarshalIndent(pol, "", "  ")
		if err != nil {
			fmt.Println(err.Error())
		}
		//	fmt.Println(string(polBytes))
		return os.WriteFile(path, polBytes, 0644)
	}

	return os.WriteFile(path, polBytes, 0644)

}
