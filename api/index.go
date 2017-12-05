package main

import (
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
    "strconv"
)

type SubRegion struct {
	Id bson.ObjectId `bson:"_id,omitempty" json:"id"`
    Name string `bson:"name" json:"name"`
    ShortName string `bson:"shortName" json:"shortName"`
    Code int `bson:"code" json:"code"`
    RegionName string `bson:"regionName" json:"regionName"`
    RegionShortName string `bson:"regionShortName" json:"regionShortName"`
    RegionCode int `bson:"regionCode" json:"regionCode"`
    State string `bson:"state" json:"state"`
    Area float64 `bson:"area" json:"area"`
    Circumference float64 `bson:"circumference" json:"circumference"`
    Geometry map[string]interface{} `bson:"geometry" json:"geometry"`
}

type Plant struct {
	Id bson.ObjectId `bson:"_id,omitempty" json:"id"`
	Type string `bson:"type" json:"type"`
    CommonName string `bson:"commonName" json:"commonName"`
    ScientificName string `bson:"scientificName" json:"scientificName"`
    HeightMin float64 `bson:"heightMin" json:"heightMin"`
    HeightMax float64 `bson:"heightMax" json:"heightMax"`
    FlowerTime []string `bson:"flowerTime" json:"flowerTime"`
    SpreadMin float64 `bson:"spreadMin" json:"spreadMin"`
    SpreadMax float64 `bson:"spreadMax" json:"spreadMax"`
    Form int `bson:"form" json:"form"`
    ImageUrl string `bson:"imageUrl" json:"imageUrl"`
    ImageBase64 string `bson:"imageBase64" json:"imageBase64"`
    Rain string `bson:"rain" json:"rain"`
    SoilTexture []string `bson:"soilTexture" json:"soilTexture"`
    SoilPh string `bson:"soilPh" json:"soilPh"`
    FlowerColor string `bson:"flowerColor" json:"flowerColor"`
}

type PlantWithCount struct {
	Id bson.ObjectId `bson:"_id,omitempty" json:"id"`
	Type string `bson:"type" json:"type"`
    CommonName string `bson:"commonName" json:"commonName"`
    ScientificName string `bson:"scientificName" json:"scientificName"`
    HeightMin float64 `bson:"heightMin" json:"heightMin"`
    HeightMax float64 `bson:"heightMax" json:"heightMax"`
    FlowerTime []string `bson:"flowerTime" json:"flowerTime"`
    SpreadMin float64 `bson:"spreadMin" json:"spreadMin"`
    SpreadMax float64 `bson:"spreadMax" json:"spreadMax"`
    Form int `bson:"form" json:"form"`
    ImageUrl string `bson:"imageUrl" json:"imageUrl"`
    ImageBase64 string `bson:"imageBase64" json:"imageBase64"`
    Rain string `bson:"rain" json:"rain"`
    SoilTexture []string `bson:"soilTexture" json:"soilTexture"`
    SoilPh string `bson:"soilPh" json:"soilPh"`
    FlowerColor string `bson:"flowerColor" json:"flowerColor"`
    Count int `bson:"count" json:"count"`
}

/*type PlantWithCount struct {
    *Plant
    Count int `bson:"count" json:"count"`
}*/

var sessionPool = make(chan *mgo.Session, 100)

func main() {
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}

	// Generate connection pool
	// TODO: Make this configurable from a config
	for i:=0; i < 100; i++ {
		sessionPool <- session.Copy()
	}
    defer session.Close()

	r := gin.Default()

	/* 
	MongoDB 3.6+
	db.occurences.aggregate([
	  { $match: { regionId: ObjectId('5a26c1d4bdb7e14648372f55')} },
	  { $sort : { count: -1 } },
	  {
		$lookup: {
		  from: "plants",
		  localField: "plantId",
		  foreignField: "_id",
		  as: "fromPlants"
	    }
	  },
	  {
	    $replaceRoot: { newRoot: { $mergeObjects: [ { $arrayElemAt: [ "$fromPlants", 0 ] }, "$$ROOT" ] } }
	  },
	  { 
	    $project: { fromPlants: 0 }
	  }
	])

	MongoDB 3.4+
	db.occurences.aggregate([
	  { $match: { regionId: ObjectId('5a26c1d4bdb7e14648372f55')} },
	  { $sort : { count: -1 } },
	  {
		$lookup: {
		  from: "plants",
		  localField: "plantId",
		  foreignField: "_id",
		  as: "fromPlants"
	    }
	  },
	  {
	    $replaceRoot: { newRoot: { $arrayToObject: { $concatArrays: [ { $objectToArray: { $arrayElemAt: [ "$fromPlants", 0 ] }}, { $objectToArray: "$$ROOT"} ] } } }
	  },
	  { 
	    $project: { fromPlants: 0 }
	  }
	])
	*/
	r.GET("/plant/location", func(c *gin.Context) {
		session := <- sessionPool
		regionsCollection := session.DB("test").C("regions")
		occurencesCollection := session.DB("test").C("occurences")
		
		parsedLatitude, _  := strconv.ParseFloat(c.Query("lat"), 64)
		parsedLongitude, _ := strconv.ParseFloat(c.Query("lng"), 64)

		// Find region based on lat/long
		foundRegion := SubRegion{}
		err = regionsCollection.Find(bson.M{"geometry": bson.M{"$geoIntersects": bson.M{"$geometry": bson.M{"type": "Point", "coordinates": []float64{parsedLongitude, parsedLatitude}}}}}).One(&foundRegion)
		if (err == mgo.ErrNotFound) {
			c.JSON(400, gin.H{
				"message": "The given location is not in Australia.",
			})
			return
		} else if (err != nil && err != mgo.ErrNotFound) {
			c.JSON(500, gin.H{
				"message": "Server error - region query.",
			})
			return
		}

		plantsWithCount := []PlantWithCount{}
		// Do the aggregation in MongoDB (let Mongo do the hard work it was designed for)
		query := []bson.M{
			{ "$match": bson.M{ "regionId": foundRegion.Id} },
			{ "$sort" : bson.M{ "count": -1 } },
			{
				"$lookup": bson.M{
					"from": "plants",
					"localField": "plantId",
					"foreignField": "_id",
					"as": "fromPlants",
				},
			},
			{
				"$replaceRoot": bson.M{ "newRoot": bson.M{ "$arrayToObject": bson.M{ "$concatArrays": []bson.M{ bson.M{"$objectToArray": bson.M{ "$arrayElemAt": []interface{}{ "$fromPlants", 0 } }}, bson.M{ "$objectToArray": "$$ROOT"} } } } },
			},
			{ 
				"$project": bson.M{ "fromPlants": 0 },
			},
		}

		pipe := occurencesCollection.Pipe(query)
		err := pipe.All(&plantsWithCount)
    	if err != nil {
        	c.JSON(500, gin.H{
				"message": "Server error - aggregation pipeline.",
			})
			return
    	}

		c.JSON(200, plantsWithCount)
	})

	// Return plants
	r.GET("/plant/search", func(c *gin.Context) {
		//session := <- sessionPool
		regionsCollection := session.DB("test").C("regions")
		occurencesCollection := session.DB("test").C("occurences")
		plantsCollection := session.DB("test").C("plants")
		
		latitude, latitudeExists := c.GetQuery("lat")
		longitude, longitudeExists := c.GetQuery("lng")
		region, regionExists := c.GetQuery("region")
		season, seasonExists := c.GetQueryArray("search-season-ckb")
		spreadMin, spreadMinExists := c.GetQuery("spread-min")
		spreadMax, spreadMaxExists := c.GetQuery("spread-max")
		heightMin, heightMinExists := c.GetQuery("height-min")
		heightMax, heightMaxExists := c.GetQuery("height-max")
		name, nameExists := c.GetQuery("plant-name-input")
		
		if (regionExists || (latitudeExists && longitudeExists)) {
			foundRegion := SubRegion{}
			if (regionExists) {
				// Find region based on name
				err = regionsCollection.Find(bson.M{"name": region}).One(&foundRegion)
				if (err == mgo.ErrNotFound) {
					c.JSON(400, gin.H{
						"message": "The given location is not in Australia.",
					})
					return
				} else if (err != nil && err != mgo.ErrNotFound) {
					c.JSON(500, gin.H{
						"message": "Server error - region query.",
					})
					return
				}
			} else {
				parsedLatitude, _  := strconv.ParseFloat(latitude, 64)
				parsedLongitude, _ := strconv.ParseFloat(longitude, 64)

				// Find region based on lat/long
				err = regionsCollection.Find(bson.M{"geometry": bson.M{"$geoIntersects": bson.M{"$geometry": bson.M{"type": "Point", "coordinates": []float64{parsedLongitude, parsedLatitude}}}}}).One(&foundRegion)
				if (err == mgo.ErrNotFound) {
					c.JSON(400, gin.H{
						"message": "The given location is not in Australia.",
					})
					return
				} else if (err != nil && err != mgo.ErrNotFound) {
					c.JSON(500, gin.H{
						"message": "Server error - region query.",
					})
					return
				}
			}
			// Get the actual plants
			//var plantsWithCount []PlantWithCount
			plantsWithCount := []PlantWithCount{}
			// Do the aggregation in MongoDB (let Mongo do the hard work it was designed for)
			query := []bson.M{
				{ "$match": bson.M{ "regionId": foundRegion.Id} },
				{ "$sort" : bson.M{ "count": -1 } },
				{
					"$lookup": bson.M{
						"from": "plants",
						"localField": "plantId",
						"foreignField": "_id",
						"as": "fromPlants",
					},
				},
				{
					"$replaceRoot": bson.M{ "newRoot": bson.M{ "$arrayToObject": bson.M{ "$concatArrays": []bson.M{ bson.M{"$objectToArray": bson.M{ "$arrayElemAt": []interface{}{ "$fromPlants", 0 } }}, bson.M{ "$objectToArray": "$$ROOT"} } } } },
				},
				{ 
					"$project": bson.M{ "fromPlants": 0 },
				},
			}

			matchQuery := bson.M{}
			if (nameExists) {
				matchQuery["scientificName"] = name
			}

			if (heightMinExists && heightMaxExists) {
				matchQuery["heightMin"] = bson.M{"$gt": heightMin}
				matchQuery["heightMax"] = bson.M{"$lt": heightMax}
			} else if(heightMaxExists){
				matchQuery["heightMax"] = bson.M{"$lt": heightMax}
			} else if(heightMinExists) {
				matchQuery["heightMin"] = bson.M{"$gt": heightMin}
			}

			if (spreadMinExists && spreadMaxExists) {
				matchQuery["spreadMin"] = bson.M{"$gt": spreadMin}
				matchQuery["spreadMax"] = bson.M{"$lt": spreadMax}
			} else if(spreadMaxExists){
				matchQuery["spreadMax"] = bson.M{"$lt": spreadMax}
			} else if(spreadMinExists) {
				matchQuery["spreadMin"] = bson.M{"$gt": spreadMin}
			}

			if (seasonExists) {
				matchQuery["flowerTime"] = bson.M{"$in": season};
			}

			query = append(query, bson.M{"$match": matchQuery})

			pipe := occurencesCollection.Pipe(query)
			//fmt.Printf("%v as", plantsWithCount)
			err := pipe.All(&plantsWithCount)
	    	if err != nil {
	        	c.JSON(500, gin.H{
					"message": "Server error - aggregation pipeline.",
				})
				return
	    	}

			c.JSON(200, plantsWithCount)		
			return
		} else {
			plants := []Plant{}
			matchQuery := bson.M{}
			if (nameExists) {
				matchQuery["scientificName"] = name
			}

			if (heightMinExists && heightMaxExists) {
				matchQuery["heightMin"] = bson.M{"$gt": heightMin}
				matchQuery["heightMax"] = bson.M{"$lt": heightMax}
			} else if(heightMaxExists){
				matchQuery["heightMax"] = bson.M{"$lt": heightMax}
			} else if(heightMinExists) {
				matchQuery["heightMin"] = bson.M{"$gt": heightMin}
			}

			if (spreadMinExists && spreadMaxExists) {
				matchQuery["spreadMin"] = bson.M{"$gt": spreadMin}
				matchQuery["spreadMax"] = bson.M{"$lt": spreadMax}
			} else if(spreadMaxExists){
				matchQuery["spreadMax"] = bson.M{"$lt": spreadMax}
			} else if(spreadMinExists) {
				matchQuery["spreadMin"] = bson.M{"$gt": spreadMin}
			}

			if (seasonExists) {
				matchQuery["flowerTime"] = bson.M{"$in": season};
			}

			err = plantsCollection.Find(matchQuery).All(&plants)
        	if err != nil {
            	c.JSON(500, gin.H{
					"message": "Server error - query.",
				})
				return
        	}

			c.JSON(200, plants)
			return
		}
	})

	// Set PORT variable to override port
	r.Run()
}
