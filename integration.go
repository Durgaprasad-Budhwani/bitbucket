package main

import (
	"github.com/pinpt/agent.next.bitbucket/internal"
	"github.com/pinpt/agent.next/runner"
)

// Integration is used to export the integration
var Integration internal.BitBucketIntegration

func main() {
	
	IntegrationDescriptor := "bmFtZTogQml0YnVja2V0CnJlZl90eXBlOiBiaXRidWNrZXQKZGVzY3JpcHRpb246IFRoaXMgaXMgdGhlIEF0bGFzc2lhbiBCaXRidWNrZXQgaW50ZWdyYXRpb24gZm9yIFBpbnBvaW50CmF2YXRhcl91cmw6IGh0dHBzOi8vaW1nLmljb25zOC5jb20vY29sb3IvMjQwLzAwMDAwMC9iaXRidWNrZXQucG5nCmNhcGFiaWxpdGllczoKICAtIHNvdXJjZWNvZGUuUmVwbwogIC0gc291cmNlY29kZS5Vc2VyCiAgLSBzb3VyY2Vjb2RlLkNvbW1pdAogIC0gc291cmNlY29kZS5QdWxsUmVxdWVzdAogIC0gc291cmNlY29kZS5QdWxsUmVxdWVzdFJldmlldwogIC0gc291cmNlY29kZS5QdWxsUmVxdWVzdENvbW1pdAogIC0gc291cmNlY29kZS5QdWxsUmVxdWVzdENvbW1lbnQKaW5zdGFsbGF0aW9uOgogIG1vZGVzOgogICAgLSBjbG91ZAogICAgLSBzZWxmbWFuYWdlZAogIHNlbGZtYW5hZ2VkOgogICAgYXV0aG9yaXphdGlvbnM6CiAgICAgIC0gb2F1dGgyCiAgY2xvdWQ6CiAgICBhdXRob3JpemF0aW9uczoKICAgICAgLSBvYXV0aDIK"
	IntegrationBuildDate := "2020-09-01T16:02:06Z"
	IntegrationBuildCommitSHA := "5dc235d22355d66ab27ab830b548cdedf326d435"
	runner.Main(&Integration, IntegrationDescriptor, IntegrationBuildDate, IntegrationBuildCommitSHA)

}

