package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func main() {
	db, err := strconv.Atoi(os.Getenv("REDIS_DATABASE"))
	if err != nil {
		log.Fatalln("error when converting db redis: " + err.Error())
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT"),
		Password: "", // no password set
		DB:       db, // use default DB
	})
	ctx := context.Background()
	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalln("redis connection error:", err)
	}

	e := echo.New()
	e.GET("/api/v1/weather", func(c echo.Context) error {
		latitude := c.QueryParam("latitude")
		longitude := c.QueryParam("longitude")
		if latitude == "" || longitude == "" {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"message": "please add latitude and longitude",
			})
		}
		key := latitude + "-" + longitude
		result, err := rdb.Get(c.Request().Context(), key).Result()
		if err != nil && err != redis.Nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"message": err.Error(),
			})
		} else if result != "" {
			var resultMap map[string]interface{}
			err = json.Unmarshal([]byte(result), &resultMap)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"message": err.Error(),
				})
			}
			return c.JSON(http.StatusOK, resultMap)
		}
		response, err := http.Get("https://api.open-meteo.com/v1/forecast?latitude=" + latitude + "&longitude=" + longitude + "&current=temperature_2m,wind_speed_10m&hourly=temperature_2m,relative_humidity_2m,wind_speed_10m")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"message": err.Error(),
			})
		}
		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"message": err.Error(),
			})
		}
		rdb.Set(c.Request().Context(), key, string(responseBody), time.Duration(12)*time.Hour)
		var responseMap map[string]interface{}
		err = json.Unmarshal(responseBody, &responseMap)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"message": err.Error(),
			})
		}
		return c.JSON(http.StatusOK, responseMap)
	})

	go func() {
		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()
	println(time.Now().String(), "echo: started at :8080")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
}
