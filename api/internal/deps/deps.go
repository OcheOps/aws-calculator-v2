// Package deps ensures Plan 02's third-party dependencies are recorded in
// go.mod before the code that uses them lands. Remove entries once real
// imports exist elsewhere in the module.
package deps

import (
	_ "github.com/aws/aws-sdk-go-v2"
	_ "github.com/aws/aws-sdk-go-v2/config"
	_ "github.com/aws/aws-sdk-go-v2/credentials"
	_ "github.com/aws/aws-sdk-go-v2/service/pricing"
	_ "github.com/robfig/cron/v3"
	_ "golang.org/x/time/rate"
)
