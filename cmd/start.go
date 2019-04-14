// Copyright © 2019 Stephen Bunn
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"fmt"
	"time"

	"github.com/scbunn/mdbload/pkg/mongo"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a load test",
	Long:  `Starts a new load test against a mongodb cluter`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("start called: " + viper.GetString("mongodb.connectionString"))
		mdb := mongo.MongoTest{
			ConnectionString: viper.GetString("mongodb.connectionString"),
			Timeout:          viper.GetDuration("mongodb.serverTimeout"),
			Database:         viper.GetString("mongodb.database"),
			Collection:       viper.GetString("mongodb.collection"),
		}
		var documents []string
		var bsonDocuments []interface{}
		documents = append(documents, "{\"name\": \"foobar\"}")
		documents = append(documents, "{\"name\": \"bob jones\"}")

		for _, doc := range documents {
			bson := mdb.ConvertJSONtoBSON(doc)
			bsonDocuments = append(bsonDocuments, bson)
		}

		fmt.Println(mdb.InsertDocuments(bsonDocuments))
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	// General flags
	startCmd.Flags().Duration("duration", 30*time.Second, "Duration of the load test")
	viper.BindPFlag("duration", startCmd.Flags().Lookup("duration"))

	// MongoDB settings
	startCmd.Flags().String("mongodb-connection-string", "mongodb://127.0.0.1:27017", "MongoDB Connection String")
	viper.BindPFlag("mongodb.connectionString", startCmd.Flags().Lookup("mongodb-connection-string"))
	startCmd.Flags().Duration("mongodb-server-timeout", 1*time.Second, "MongoDB server connection timeout")
	viper.BindPFlag("mongodb.serverTimeout", startCmd.Flags().Lookup("mongodb-server-timeout"))
	startCmd.Flags().String("mongodb-database", "loadtest", "Database to use for load tests")
	viper.BindPFlag("mongodb.database", startCmd.Flags().Lookup("mongodb-database"))
	startCmd.Flags().String("mongodb-collection", "samples", "Collection to use for load tests")
	viper.BindPFlag("mongodb.collection", startCmd.Flags().Lookup("mongodb-collection"))

}
