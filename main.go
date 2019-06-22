package main

import (
	"log"
	"strconv"
	"strings"

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
	Expiring         int
	UserId           string
}

func reminderHandler(req events.CloudWatchEvent) error {
	dynamoProperties, err := properties.ReadDynamoProperties(DATABASE_PROPERTIES)
	userTableName := dynamoProperties.GetTableName("userData")
	tempReservationTableName := dynamoProperties.GetTableName("tempReservation")

	if err != nil {
		panic("Failed to read database properties")
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
		panic("Failed to fetch temporart reservations")
	}

	for _, reservation := range tempReservations.Items {
		reservationItem := ReservationDynamoModel{}

		err = dynamodbattribute.UnmarshalMap(reservation, &reservationItem)
		if err != nil {
			panic("Failed to unmarshall reservation record")
		}

		proj := expression.NamesList(expression.Name("FullName"), expression.Name("Email"), expression.Name("Phone"), expression.Name("UserId"))
		result, err := dynamo.CustomQuery("ReservationId", reservationItem.ReservationID, userTableName, proj)
		if err != nil {
			panic("Failed to fetch user data")
		}

		for _, i := range result.Items {
			item := dynamo.UserModel{}

			err = dynamodbattribute.UnmarshalMap(i, &item)
			if err != nil {
				panic("Failed to unmarshall user data record")
			}

			templateBytes := utils.ReadBytesFromFile(EMAIL_TEMPLATE)
			tempateString := string(templateBytes)

			r := strings.NewReplacer(
				"<cost>", reservationItem.CostValue,
				"<depositCost>", reservationItem.DepositCostValue,
				"<reservationId>", reservationItem.ReservationID,
				"<expiration>", strconv.Itoa(reservationItem.Expiring))

			err = SendTransactionalMail(item.Email, "Foglalásod hamarosan törlésre kerül", r.Replace(tempateString))
			if err != nil {
				panic("Failed to send transactional email")
			}

		}
	}

	return nil
}

func main() {
	lambda.Start(reminderHandler)
}
