package database

import (
	"EventHunting/configs"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBConfig struct {
	Name string
	URI  string
}

func NewMongoDBConfig() *MongoDBConfig {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	return &MongoDBConfig{
		Name: configs.GetDatabaseName(),
		URI:  configs.GetDatabaseURI(),
	}
}

var (
	client    *mongo.Client
	db        *mongo.Database
	mongoOnce sync.Once
)

func ConnectMongo() error {
	var err error
	mongoOnce.Do(func() {
		mongoDBConfig := NewMongoDBConfig()
		fmt.Println("Running")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoDBConfig.URI))
		if err != nil {
			fmt.Errorf("lỗi khi kết nối MongoDB: %v", err)
		}

		err = client.Ping(ctx, nil)
		if err != nil {
			fmt.Errorf("không thể ping tới MongoDB: %v", err)
		}

		fmt.Println("Đã kết nối MongoDB thành công!")
		db = client.Database(mongoDBConfig.Name)
	})

	return err
}

func GetDB() *mongo.Database {
	return db
}
