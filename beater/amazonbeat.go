package beater

import (
	"fmt"
	"time"
	"strconv"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/felix-lessoer/amazonbeat/config"

	"github.com/ngs/go-amazon-product-advertising-api/amazon"
)

type Amazonbeat struct {
	done   chan struct{}
	config config.Config
	client beat.Client
}

// Creates beater
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &Amazonbeat{
		done:   make(chan struct{}),
		config: c,
	}
	return bt, nil
}

func getPrice(price string) (float64) {
	retPrice, err := strconv.ParseFloat(price, 64)
	if err != nil {
		logp.Info("Error at parsing")
	}
	retPrice = retPrice / 100
	return retPrice
}

func (bt *Amazonbeat) readAmazonData(aClient *amazon.Client, asins []string, lookupSimilarProducts bool) ([]amazon.Item, error) {
	logp.Info("Load via API")

	res, err := aClient.ItemLookup(amazon.ItemLookupParameters{
		ResponseGroups: []amazon.ItemLookupResponseGroup{
			amazon.ItemLookupResponseGroupLarge,
		},
		IDType: amazon.IDTypeASIN,
		ItemIDs: asins,
	}).Do()


	if err != nil {
		logp.Error(err)
	}

	if res != nil {
		response := res.Items.Item
		//Do another lookup for similar products
		if (lookupSimilarProducts) {
			for _, item := range res.Items.Item {
				asinsForLookup := make([]string, 10)
				for _, simItem := range item.SimilarProducts.SimilarProduct {
					asinsForLookup = append(asinsForLookup, simItem.ASIN)
				}
				secondResponse, err := bt.readAmazonData(aClient, asinsForLookup, false)
				if err != nil {
					logp.Error(err)
				}
				for _, newItem := range secondResponse {
					response = append(response, newItem)
				}
			}
		}
		return response, err
	}

	return nil, nil
}

func (bt *Amazonbeat) Run(b *beat.Beat) error {
	logp.Info("amazonbeat is running! Hit CTRL-C to stop it.")

	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	//Connect to AWS
	aClient, err := amazon.New(bt.config.AWS_ACCESS_KEY_ID,bt.config.AWS_SECRET_ACCESS_KEY,bt.config.AWS_ASSOCIATE_TAG, amazon.Region(bt.config.AWS_PRODUCT_REGION))
	if err != nil {
		logp.Error(err)
	}

	ticker := time.NewTicker(bt.config.Period)
	for {
		select {
		case <-bt.done:
			return nil
		case <-ticker.C:
		}

		asinCollector := make([]string, 10)
		asinsForLookup := make([]string, 10)
		for _, asin := range bt.config.ASIN {

			if len(asinCollector) < 10 {
				asinCollector = append(asinCollector, asin)
				continue
			} else {
				asinsForLookup = asinCollector
				asinCollector = asinCollector[:0]
			}

			productData, err := bt.readAmazonData(aClient, asinsForLookup, true)
			if err != nil {
				return err
			}

			if productData == nil {
				for _, nonValidAsin := range asinsForLookup {
					logp.Info(nonValidAsin + " is not valid in given context")
				}
			}

			for _, item := range productData {
				if (item.ItemAttributes.ListPrice.Amount != "") {
					originalprice := getPrice(item.ItemAttributes.ListPrice.Amount)
					saleprice := getPrice(item.OfferSummary.LowestNewPrice.Amount)

					event := beat.Event{
						Timestamp: time.Now(),
						Fields: common.MapStr{
							"type":          b.Info.Name,//
							"product":       item.ItemAttributes.Title,
							"saleprice":     saleprice,
							"originalprice": originalprice,
							"asin":          asin,
							"image":				 item.MediumImage.URL,
							"URL":					 item.DetailPageURL,
							"category":			 item.ItemAttributes.ProductTypeName,
							//"numreviews":    numReviews,
							//"rating":        rating,
						},
					}

					bt.client.Publish(event)
					logp.Info("Event sent")
				}
			}
		}
	}
}

func (bt *Amazonbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
