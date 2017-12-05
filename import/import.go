package main

import (
	"fmt"
	//"log"
	//"github.com/benmanns/goworker"
	"gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
    "encoding/json"
	"log"
    "os"
    "strings"
    "strconv"
    "github.com/briandowns/spinner"
    "time"
    "encoding/base64"
    "io/ioutil"
    "net/http"
    //"io"
    "net/url"
)

type SubRegion struct {
	Id bson.ObjectId `bson:"_id,omitempty" json:"id"`
    Name string `bson:"name" json:"name"`
    //Polygon GeoJsonPolygon `bson:"polygon" json:"polygon"`
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

// TODO: Use Fauna in future
/*type Fauna struct {
	Id bson.ObjectId `bson:"_id,omitempty" json:"id"`
    CommonName string `bson:"commonName" json:"commonName"`
    ScientificName string `bson:"scientificName" json:"scientificName"`
}*/

// What this script does is
// 1. Populates the "regions" collection with IBRA subregions
// 2. Populates the "plants" collection with SA State flora plant data
// 3. Populates the "occurences" (by matching location to the IBRA subregions)
// 4. Done! :)

var sessionPool = make(chan *mgo.Session, 100)
var alaClient = &http.Client{Timeout: 10 * time.Second}

func importRegions(s *spinner.Spinner) {
	//s.Suffix = fmt.Sprintf(" Importing regions...\n")
	session := <- sessionPool
	session.SetSocketTimeout(1 * time.Hour)

	// Clear the regions collection first
	session.DB("test").C("regions").DropCollection()

	regionsCollection := session.DB("test").C("regions")

	raw, err := os.Open("../data/IBRASubegion_Aust70-cleaned.geojson")
	if err != nil {
        fmt.Println(err.Error())
        os.Exit(1)
    }

    dec := json.NewDecoder(raw)

    // Read open bracket
	_, err = dec.Token()
	if err != nil {
		log.Fatal(err)
	}

	// While the array contains values
	for dec.More() {
		var ibraSubRegion map[string]interface{}
		// Decode a single location
		err := dec.Decode(&ibraSubRegion)
		if err != nil {
			log.Fatal(err)
		}

		transformedSubRegion := SubRegion{
			Name: ibraSubRegion["properties"].(map[string]interface{})["IBRA_SUB_N"].(string),
			ShortName: ibraSubRegion["properties"].(map[string]interface{})["IBRA_SUB_C"].(string),
			Code: int(ibraSubRegion["properties"].(map[string]interface{})["IBRA_SUB_1"].(float64)),
			RegionName: ibraSubRegion["properties"].(map[string]interface{})["IBRA_REG_N"].(string),
			RegionShortName: ibraSubRegion["properties"].(map[string]interface{})["IBRA_REG_C"].(string),
			RegionCode: int(ibraSubRegion["properties"].(map[string]interface{})["IBRA_REG_1"].(float64)),
			State: ibraSubRegion["properties"].(map[string]interface{})["STATE"].(string),
			Area: ibraSubRegion["properties"].(map[string]interface{})["SHAPE_AREA"].(float64),
			Circumference: ibraSubRegion["properties"].(map[string]interface{})["SHAPE_LEN"].(float64),
			Geometry: ibraSubRegion["geometry"].(map[string]interface{})}

		regionsCollection.Insert(transformedSubRegion)

		s.Suffix = fmt.Sprintf(" Importing regions...%v", transformedSubRegion.Name)
	}

	// Read closing bracket
	_, err = dec.Token()
	if err != nil {
		log.Fatal(err)
	}
	
	s.Suffix = fmt.Sprintf(" Building index on regions collection (geometry key - 2dsphere index)...this may take a while")
	err = regionsCollection.EnsureIndexKey("$2dsphere:geometry")
	if err != nil {
		log.Fatal(err)
	} else {
		log.Fatal("Index ensured")
	}

	sessionPool <- session
}

func getThumbnailLinkAndBase64(scientificName string) (string, string, error) {
	var alaBaseUrl = "http://bie.ala.org.au/ws/search.json?q="
	var imageUrl string = ""
    var encodedImage string = ""
    var plantUrl string = alaBaseUrl + url.QueryEscape(scientificName)
    r, err := alaClient.Get(plantUrl)
    if err != nil {
        return imageUrl, encodedImage, err
    }
    defer r.Body.Close()

    var result map[string]interface{}

    err = json.NewDecoder(r.Body).Decode(&result)
    // TODO: Handle missing ALA plant!
    var foundPlant = result["searchResults"].(map[string]interface{})["results"].([]interface{})[0].(map[string]interface{})
    if (foundPlant["scientificName"].(string) == scientificName) {
    	imageUrlUntyped, present := foundPlant["imageUrl"]
    	if (present) {
    		imageUrl = imageUrlUntyped.(string)

	    	// Download image and encode it!
	    	imageResponse, err := alaClient.Get(imageUrl)
			if err != nil {
				log.Fatal(err)
			    return imageUrl, encodedImage, err
			}
			defer imageResponse.Body.Close()

			body, err := ioutil.ReadAll(imageResponse.Body)
			if (err != nil) {
				log.Fatal(err)
			}

	    	encodedImage = base64.StdEncoding.EncodeToString(body)
	    }
    }

    return imageUrl, encodedImage, err
}

func importPlants(s *spinner.Spinner) {
	// Plants import
	//s.Suffix = fmt.Sprintf(" Importing plants...\n")
	session := <- sessionPool

	// Clear the plants collection first
	session.DB("test").C("plants").DropCollection()

	plantsCollection := session.DB("test").C("plants")

	raw, err := os.Open("../data/stateflora.json")
	if err != nil {
        fmt.Println(err.Error())
        os.Exit(1)
    }

    dec := json.NewDecoder(raw)

    // Read open bracket
	_, err = dec.Token()
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Printf("Open bracket %T: %v\n", t, t)

	// While the array contains values
	for dec.More() {
		var plant map[string]interface{}
		// Decode a single location
		err := dec.Decode(&plant)
		if err != nil {
			log.Fatal(err)
		}

		var heightMin float64
		var heightMax float64
		switch plant["Height (m)"].(type){
		  	case float64: 
		    	heightMin = plant["Height (m)"].(float64)
				heightMax = plant["Height (m)"].(float64)
		    case string:
		    	heights := strings.Split(plant["Height (m)"].(string), "-")
				heightMin, _ = strconv.ParseFloat(heights[0], 64)
				if (len(heights) > 1) {
					heightMax, _ = strconv.ParseFloat(heights[1], 64)
				} else {
					heightMax, _ = strconv.ParseFloat(heights[0], 64)
				}
		}
		var spreadMin float64
		var spreadMax float64
		switch plant["Spread (m)"].(type){
		  	case float64: 
		    	spreadMin = plant["Spread (m)"].(float64)
				spreadMax = plant["Spread (m)"].(float64)
		    case string:
		    	spreads := strings.Split(plant["Spread (m)"].(string), "-")
				spreadMin, _ = strconv.ParseFloat(spreads[0], 64)
				if (len(spreads) > 1) {
					spreadMax, _ = strconv.ParseFloat(spreads[1], 64)
				} else {
					spreadMax, _ = strconv.ParseFloat(spreads[0], 64)
				}
		}

		flowerTime := strings.Split(plant["Flower (time)"].(string), ",")
		
		var rainAmount string
		switch plant["Rain (mm)"].(type){
		  	case float64: 
		    	rainAmount = strconv.FormatInt(int64(plant["Rain (mm)"].(float64)), 10)
		    case string:
		    	rainAmount = plant["Rain (mm)"].(string)
		}

		soilTexture := strings.Split(plant["Soil texture"].(string), ",")

		scientificName := plant["Genus"].(string) + " " + plant["Botanical name"].(string)

		imageUrl, imageBase64, err := getThumbnailLinkAndBase64(scientificName)

		transformedPlant := Plant{
			Type: plant["Type"].(string),
			CommonName: plant["Common name"].(string),
			ScientificName: scientificName,
			HeightMin: heightMin,
			HeightMax: heightMax,
			SpreadMin: spreadMin,
			SpreadMax: spreadMax,
			FlowerTime: flowerTime,
			Form: 1,
			Rain: rainAmount,
			SoilTexture: soilTexture,
			SoilPh: plant["Soil pH"].(string),
			FlowerColor: plant["Flower colour"].(string),
			ImageUrl: imageUrl,
			ImageBase64: imageBase64}

		plantsCollection.Insert(transformedPlant)

		s.Suffix = fmt.Sprintf(" Importing plants...%v", transformedPlant.CommonName)
	}

	// Read closing bracket
	_, err = dec.Token()
	if err != nil {
		log.Fatal(err)
	}

	sessionPool <- session
}

// TODO: Parallelize this - urgent!
func importOccurences(s *spinner.Spinner) {
	session := <- sessionPool
	regionsCollection := session.DB("test").C("regions")
	plantsCollection := session.DB("test").C("plants")
	occurencesCollection := session.DB("test").C("occurences")

	// Clear the regions collection first
	session.DB("test").C("occurences").RemoveAll(nil)

	raw, err := os.Open("../data/Plants-brief.json")
	if err != nil {
        fmt.Println(err.Error())
        os.Exit(1)
    }

    dec := json.NewDecoder(raw)

    // Read open bracket
	_, err = dec.Token()
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Printf("Open bracket %T: %v\n", t, t)

	var importedCount int = 0
	var skippedCount int = 0

	// While the array contains values
	for dec.More() {
		var occurence map[string]interface{}
		var foundPlant map[string]interface{}
		var foundRegion map[string]interface{}
		var insertedOccurence map[string]interface{}
		// Decode a single occurence
		err := dec.Decode(&occurence)
		if err != nil {
			log.Fatal(err)
		}

		// Find the plant for this name - TODO: Cache?
		err = plantsCollection.Find(bson.M{"scientificName": occurence["scientificName"].(string)}).One(&foundPlant)
		if (err == mgo.ErrNotFound) {
			// Skip
			skippedCount++
			continue
		} else if (err != nil && err != mgo.ErrNotFound) {
			log.Fatal(err)
		}
		parsedLongitude, _ := strconv.ParseFloat(occurence["decimalLongitude"].(string), 64)
		parsedLatitude, _ := strconv.ParseFloat(occurence["decimalLatitude"].(string), 64)

		// Find the region for this location - TODO: Cache?
		err = regionsCollection.Find(bson.M{"geometry": bson.M{"$geoIntersects": bson.M{"$geometry": bson.M{"type": "Point", "coordinates": []float64{parsedLongitude, parsedLatitude}}}}}).One(&foundRegion)
		if (err == mgo.ErrNotFound) {
			// Skip
			skippedCount++
			continue
		} else if (err != nil && err != mgo.ErrNotFound) {
			log.Fatal(err)
		}

		//fmt.Printf("foundPlant %v \n", foundPlant["scientificName"].(string))
		//fmt.Printf("foundRegion %v \n", foundRegion["name"].(string))

		plantId := foundPlant["_id"].(bson.ObjectId)
		regionId := foundRegion["_id"].(bson.ObjectId)

		change := mgo.Change{
	        Update: bson.M{
			    "$setOnInsert": bson.M{"plantId": plantId, "regionId": regionId},
			    "$inc": bson.M{"count": 1}},
	        ReturnNew: true,
	        Upsert: true}
		_, err = occurencesCollection.Find(bson.M{"regionId": regionId, "plantId": plantId}).Apply(change, &insertedOccurence)
		if (err == mgo.ErrNotFound) {
			// Skip
			continue
		} else if (err != nil && err != mgo.ErrNotFound) {
			log.Fatal(err)
		}
		//fmt.Println(insertedOccurence)

		s.Suffix = fmt.Sprintf(" Importing occurences...%v complete, %v skipped", importedCount, skippedCount)
		importedCount++
	}

	// Read closing bracket
	_, err = dec.Token()
	if err != nil {
		log.Fatal(err)
	}

	sessionPool <- session
}

func main() {
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}

	// Generate connection pool
	for i:=0; i < 100; i++ {
		sessionPool <- session.Copy()
	}
    defer session.Close()

    // 1. Import regions
	// Spinner
	sRegions := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	sRegions.Writer = os.Stderr
	sRegions.Start()
	sRegions.Suffix = " Importing regions..."
	sRegions.FinalMSG = "Importing regions...complete!\n"
    importRegions(sRegions)
    sRegions.Stop()
    time.Sleep(100 * time.Millisecond) 

    // 2. Import plants
    sPlants := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	sPlants.Writer = os.Stderr
	sPlants.Start()
	sPlants.Suffix = " Importing plants..."
	sPlants.FinalMSG = "Importing plants...complete!\n"
    importPlants(sPlants)
    sPlants.Stop()
    time.Sleep(100 * time.Millisecond)  
    
    // 3. Import occurences and aggregate them
    sOccurences := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	sOccurences.Writer = os.Stderr
	sOccurences.Start()
	sOccurences.Suffix = " Importing occurences..."
	sOccurences.FinalMSG = "Importing occurences...complete!\n"
    importOccurences(sOccurences)
    sOccurences.Stop()
    time.Sleep(100 * time.Millisecond) 
}