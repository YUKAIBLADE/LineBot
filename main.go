package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/line/line-bot-sdk-go/linebot"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserMessage struct {
	UserID  string    `bson:"user_id"`
	Message string    `bson:"message"`
	Time    time.Time `bson:"time"`
}

func main() {
	// setup gin framework
	r := gin.Default()

	// setup line-bot-sdk-go client
	bot, err := linebot.New("9b2f5da495af3894bbb15004e1abf834", "x4aEQA+icyKiIGYbKoqrt6kZi3PaHPToy0efB3h6hjNVWhZIXtlH9jSqLU/9MmDJTVtoNgqkUBH+vxpJ2h0GDpwnFFcIxyqqOVnVppfmZqtdVhxWetJ4hQrA/GR2JP6YM1kGjdptvILYCr1TMa7BAAdB04t89/1O/w1cDnyilFU=")

	if err != nil {
		log.Fatalf("linebot.New error: %s", err)
	}

	uri := "mongodb://admin:123456@localhost:27017"

	// 建立連線到 MongoDB 的 client
	client, err := mongo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	r.GET("/messages", func(c *gin.Context) {
		collection := client.Database("test").Collection("messages")

		var messages []UserMessage
		cur, err := collection.Find(ctx, bson.M{})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		defer cur.Close(ctx)

		for cur.Next(ctx) {
			var message UserMessage
			err := cur.Decode(&message)
			if err != nil {
				log.Fatal(err)
			}
			messages = append(messages, message)
		}

		c.JSON(http.StatusOK, messages)
	})

	// setup line webhook
	r.POST("/callback", func(c *gin.Context) {
		events, err := bot.ParseRequest(c.Request)

		if err != nil {
			if err == linebot.ErrInvalidSignature {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		for _, event := range events {

			// 創建 collection
			collection := client.Database("test").Collection("messages")

			if event.Type == linebot.EventTypeMessage {
				switch message := event.Message.(type) {

				case *linebot.TextMessage:

					// 新增訊息
					newMessage := UserMessage{
						UserID:  event.Source.UserID,
						Message: message.Text,
						Time:    time.Now(),
					}
					_, err = collection.InsertOne(ctx, newMessage)
					if err != nil {
						log.Fatal(err)
					}

					// quota, err := bot.GetMessageQuota().Do()
					// if err != nil {
					// 	log.Println("err:", err)
					// }

					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("收到文字了訊息，感謝您")).Do(); err != nil {
						log.Print(err)
					}

				case *linebot.StickerMessage:
					var kw string
					for _, k := range message.Keywords {
						kw = kw + "," + k
					}

					outStickerResult := fmt.Sprintf("收到貼圖訊息，感謝您")
					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(outStickerResult)).Do(); err != nil {
						log.Print(err)
					}
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Message saved successfully!"})
	})

	r.Run("localhost:8080")
}
