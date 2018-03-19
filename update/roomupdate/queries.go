package roomupdate

import (
	"github.com/byuoitav/state-parsing/common"
)

var RoomUpdateQuery = &common.ElkQuery{
	Method:   "POST",
	Endpoint: "/oit-static-av-devices,oit-static-av-rooms/_search",
	Query: `{
"_source": false,
  "query": {
    "query_string": {
      "query": "*"
    }
  },
  "aggs": {
    "rooms": {
      "terms": {
        "field": "room",
        "size": 1000
      },
      "aggs": {
        "index": {
          "terms": {
            "field": "_index"
          },
          "aggs": {
            "alerting": {
              "terms": {
                "field": "alerting"
              },
              "aggs": {
                "device-name": {
                  "terms": {
                    "field": "hostname",
                    "size": 100
                  }
                }
              }
            },
            "power": {
              "terms": {
                "field": "power"
              },
              "aggs": {
                "device-name": {
                  "terms": {
                    "field": "hostname",
                    "size": 100
                  }
                }
              }
            }
          }
        }
      }
    }
  },
  "size": 0
}`,
}

/*
type AutoGenerated struct {
	Aggregations struct {
		Rooms struct {
			Buckets []struct {
				Key      string `json:"key"`
				DocCount int    `json:"doc_count"`
				Index    struct {
					Buckets []struct {
						Bucket
						Key      string `json:"key"`
						DocCount int    `json:"doc_count"`
						Power    struct {
							Buckets []struct {
								Key        string `json:"key"`
								DocCount   int    `json:"doc_count"`
								DeviceName struct {
									Buckets []struct {
										Key      string `json:"key"`
										DocCount int    `json:"doc_count"`
									} `json:"buckets"`
								} `json:"device-name"`
							} `json:"buckets"`
						} `json:"power"`
						Alerting struct {
							DocCountErrorUpperBound int `json:"doc_count_error_upper_bound"`
							SumOtherDocCount        int `json:"sum_other_doc_count"`
							Buckets                 []struct {
								Key         int    `json:"key"`
								KeyAsString string `json:"key_as_string"`
								DocCount    int    `json:"doc_count"`
								DeviceName  struct {
									DocCountErrorUpperBound int `json:"doc_count_error_upper_bound"`
									SumOtherDocCount        int `json:"sum_other_doc_count"`
									Buckets                 []struct {
										Key      string `json:"key"`
										DocCount int    `json:"doc_count"`
									} `json:"buckets"`
								} `json:"device-name"`
							} `json:"buckets"`
						} `json:"alerting"`
					} `json:"buckets"`
				} `json:"index"`
			} `json:"buckets"`
		} `json:"rooms"`
	} `json:"aggregations"`
}
*/

type RoomQueryResponse struct {
	Aggregations struct {
		Rooms Field `json:"rooms"`
	} `json:"aggregations"`
}

type Bucket struct {
	Key      string `json:"key"`
	DocCount int    `json:"doc_count"`
	Field    map[string]Field
}

type Field struct {
	Buckets []Bucket `json:"buckets"`
}
