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
	"io/ioutil"
	"os"
	"strings"
	"time"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/onrik/logrus/filename"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var (
	VERSION   string
	GITSHA    string
	BUILDTIME string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mdbload",
	Short: "MongoDB Load Testing Tool",
	Long: `Load test a mongodb cluster by generating read/write load against a cluster for a specific duration.

mdbload can run locally for small tests; however, for large tests mdbload is designed to scale by distributing load
generation through a kubernetes cluster.  For more information on configuration and operation please see the provided
documentation.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version string, sha string, date string) {
	VERSION = version
	GITSHA = sha
	BUILDTIME = date
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.mdbload.yaml)")

	// MongoDB settings
	rootCmd.PersistentFlags().String("mongodb-connection-string", "mongodb://127.0.0.1:27017", "MongoDB Connection String")
	rootCmd.PersistentFlags().String("mongodb-database", "loadtest", "Database to use for load tests")
	rootCmd.PersistentFlags().String("mongodb-collection", "samples", "Collection to use for load tests")
	rootCmd.PersistentFlags().String("mongodb-read-preference", "Primary", "mongodb read preference (Primary|PrimaryPreffered|Secondary|SecondaryPreferred|Nearest)")
	rootCmd.PersistentFlags().Bool("mongodb-write-journal", true, "request ack from mongodb that write operations have been written to the journal")
	rootCmd.PersistentFlags().Uint8("mongodb-write-concern", 1, "The number of mongo servers that much acknowledge a write")
	rootCmd.PersistentFlags().Duration("mongodb-connection-timeout", 10*time.Second, "MongoDB initial server connection timeout")
	rootCmd.PersistentFlags().Duration("mongodb-server-selection-timeout", 10*time.Second, "MongoDB server selection timeout")
	rootCmd.PersistentFlags().Duration("mongodb-socket-timeout", 1*time.Second, "MongoDB operation timeout")
	rootCmd.PersistentFlags().Uint16("mongodb-connection-pool-size", 100, "Size of the mongodb connection pool")

	// logging
	rootCmd.PersistentFlags().Bool("logging-enable", false, "enable output logging")
	rootCmd.PersistentFlags().Bool("logging-source", false, "enable source file logging field")
	rootCmd.PersistentFlags().String("logging-level", "info", "logging level (debug,info,warn,error)")
	rootCmd.PersistentFlags().String("logging-format", "text", "logging output format (text|json)")

	viper.BindPFlag("mongodb.connectionString", rootCmd.PersistentFlags().Lookup("mongodb-connection-string"))
	viper.BindPFlag("mongodb.database", rootCmd.PersistentFlags().Lookup("mongodb-database"))
	viper.BindPFlag("mongodb.collection", rootCmd.PersistentFlags().Lookup("mongodb-collection"))
	viper.BindPFlag("mongodb.readPreference", rootCmd.PersistentFlags().Lookup("mongodb-read-preference"))
	viper.BindPFlag("mongodb.writeConcern", rootCmd.PersistentFlags().Lookup("mongodb-write-concern"))
	viper.BindPFlag("mongodb.writeJournal", rootCmd.PersistentFlags().Lookup("mongodb-write-journal"))
	viper.BindPFlag("mongodb.connectionPoolSize", rootCmd.PersistentFlags().Lookup("mongodb-connection-pool-size"))
	viper.BindPFlag("logging.enable", rootCmd.PersistentFlags().Lookup("logging-enable"))
	viper.BindPFlag("logging.level", rootCmd.PersistentFlags().Lookup("logging-level"))
	viper.BindPFlag("logging.format", rootCmd.PersistentFlags().Lookup("logging-format"))
	viper.BindPFlag("logging.source", rootCmd.PersistentFlags().Lookup("logging-source"))

}

// configureLogging configures a new logrus logger
func configureLogging() {
	lvl := viper.GetString("logging.level")
	enable := viper.GetBool("logging.enable")
	format := viper.GetString("logging.format")
	enableSource := viper.GetBool("logging.source")

	// if logging is disabled nothing else matters
	if !enable {
		log.SetOutput(ioutil.Discard)
		return
	}

	// set logging level
	l, err := log.ParseLevel(lvl)
	if err != nil {
		log.WithField("level", lvl).Warn("Invalid level, failling back to 'info'")
	} else {
		log.SetLevel(l)
	}

	// set logging format
	switch format {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	case "text":
		log.SetFormatter(&log.TextFormatter{
			DisableColors: false,
			FullTimestamp: true,
		})
	default:
		log.WithField("format", format).Warn("Invalid format, defaulting to text")
		log.SetFormatter(&log.TextFormatter{
			DisableColors: false,
			FullTimestamp: true,
		})
	}

	// Enable/disable source file logging
	if enableSource {
		log.AddHook(filename.NewHook())
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".mdbload" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".mdbload")
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}

	// configure logging
	configureLogging()
}
