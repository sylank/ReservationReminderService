package main

import (
	"log"
	"strings"
	"time"

	"github.com/sylank/lavender-commons-go/utils"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/sylank/lavender-commons-go/dynamo"
	"github.com/sylank/lavender-commons-go/properties"
)

const (
	EMAIL_TEMPLATE      = "./config/deletion_reminder_template.html"
	DATABASE_PROPERTIES = "./config/database_properties.json"
)

// ReservationDynamoModel ...
type ReservationDynamoModel struct {
	ReservationID    string
	CostValue        string
	DepositCostValue string
	Expiring         int64
	UserId           string
}

func reminderHandler(req events.CloudWatchEvent) error {
	dynamoProperties, err := properties.ReadDynamoProperties(DATABASE_PROPERTIES)
	userTableName := dynamoProperties.GetTableName("userData")
	tempReservationTableName := dynamoProperties.GetTableName("tempReservation")
	log.Println("Table names:")
	log.Println(userTableName)
	log.Println(tempReservationTableName)

	dynamo.CreateConnection(dynamoProperties)

	if err != nil {
		log.Println("Failed to read database properties")
		panic(err)
	}

	log.Println("Query reservations from temporary table")

	proj := expression.NamesList(
		expression.Name("CostValue"),
		expression.Name("DepositCostValue"),
		expression.Name("Expiring"),
		expression.Name("ReservationId"),
		expression.Name("UserId"),
	)
	tempReservations, err := dynamo.FetchTable(tempReservationTableName, proj)
	if err != nil {
		log.Println("Failed to fetch temporary reservations")
		panic(err)
	}

	for _, reservation := range tempReservations.Items {
		reservationItem := ReservationDynamoModel{}

		err = dynamodbattribute.UnmarshalMap(reservation, &reservationItem)
		if err != nil {
			log.Println("Failed to unmarshall reservation record")
			panic(err)
		}

		log.Println("Reservation item:")
		log.Println(reservationItem)

		proj := expression.NamesList(expression.Name("FullName"), expression.Name("Email"), expression.Name("Phone"), expression.Name("UserId"))
		result, err := dynamo.CustomQuery("UserId", reservationItem.UserId, userTableName, proj)
		if err != nil {
			log.Println("Failed to fetch user data")
			panic(err)
		}

		for _, i := range result.Items {
			item := dynamo.UserModel{}

			err = dynamodbattribute.UnmarshalMap(i, &item)
			if err != nil {
				log.Println("Failed to unmarshall user data record")
				panic(err)
			}

			log.Println("User item:")
			log.Println(item)

			log.Println("Sending transactional mail")
			templateBytes := utils.ReadBytesFromFile(EMAIL_TEMPLATE)
			tempateString := string(templateBytes)

			r := strings.NewReplacer(
				"<cost>", reservationItem.CostValue,
				"<depositCost>", reservationItem.DepositCostValue,
				"<reservationId>", reservationItem.ReservationID,
				"<expiration>", convertTimestampToReadable(reservationItem.Expiring))

			err = SendTransactionalMail(item.Email, "Foglalásod hamarosan törlésre kerül", r.Replace(tempateString))
			if err != nil {
				log.Println("Failed to send transactional email")
				panic(err)
			}

		}
	}

	return nil
}

func convertTimestampToReadable(timestamp int64) string {
	unixTimeUTC := time.Unix(timestamp, 0)

	return unixTimeUTC.Format(time.RFC3339)
}

func main() {
	lambda.Start(reminderHandler)
}
