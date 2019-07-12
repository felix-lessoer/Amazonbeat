// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import "time"

type Config struct {
	Period time.Duration `config:"period"`
	ASIN []string `config:"asin"`
	AWS_ACCESS_KEY_ID string `config:"accessKeyID"`
	AWS_SECRET_ACCESS_KEY string `config:"secretAccessKey"`
	AWS_ASSOCIATE_TAG string `config:"associateTag"`
	AWS_PRODUCT_REGION string `config:"productRegion"`
}

var DefaultConfig = Config{
	Period: 1 * time.Second,
	ASIN: []string{"B01MDLTVT5"},
}
