package metadata

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const ResumeTokenDirectory = "app/data"
const ResumeTokenFile = "resume_token.bin"

var (
	client              *mongo.Client
	ResumeTokenFilePath = filepath.Join(ResumeTokenDirectory, ResumeTokenFile)
)

func MiningFileMetadata(ctx context.Context) error {
	fmt.Println("Mining file metadata...")

	if err := initializeMongoClient(ctx); err != nil {
		return err
	}
	defer disconnectMongoClient()

	if err := CreateEventProducer(); err != nil {
		return err
	}
	defer CloseEventProducer()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Context cancelled, mining file metadata stopped")
			return ctx.Err()
		default:
			if err := WatchChangeStream(ctx); err != nil {
				if ctx.Err() != nil && (ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded) {
					return ctx.Err()
				}

				return fmt.Errorf("failed to watch file metadata: %v", err)
			}
		}
	}
}

func WatchChangeStream(ctx context.Context) error {
	fmt.Println("Watching change stream for file metadata...")
	if err := EnsureResumeTokenDirectoryExists(); err != nil {
		return fmt.Errorf("failed to create resume token directory: %v", err)
	}

	resumeToken, err := FetchResumeToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch resume token: %v", err)
	}

	collection := client.Database("store_file").Collection("file")
	changeStreamOptions := options.ChangeStream()
	changeStreamOptions = ResumeChangeStreamIfPossible(resumeToken, changeStreamOptions)

	return WatchChengeStreamEvents(ctx, collection, changeStreamOptions)
}

func WatchChengeStreamEvents(ctx context.Context, collection *mongo.Collection, changeStreamOptions *options.ChangeStreamOptions) error {
	changeStream, err := collection.Watch(ctx, mongo.Pipeline{}, changeStreamOptions)
	if err != nil {
		return fmt.Errorf("failed to watch change stream: %v", err)
	}
	defer changeStream.Close(ctx)

	for changeStream.Next(ctx) {
		var change bson.M
		if err := changeStream.Decode(&change); err != nil {
			return fmt.Errorf("failed to decode change stream event: %w", err)
		}
		fmt.Printf("Change detected: %v\n", change)
		_, err := CreateFileStoredEvent(change)
		if err != nil {
			return fmt.Errorf("failed to create file stored event: %w", err)
		}
		resumeToken := changeStream.ResumeToken()
		err = StoreResumeToken(ctx, resumeToken)
		if err != nil {
			return fmt.Errorf("failed to store resume token: %w", err)
		}
	}

	if err := changeStream.Err(); err != nil {
		return fmt.Errorf("error in change stream: %v", err)
	}
	return nil
}

func ResumeChangeStreamIfPossible(resumeToken bson.Raw, changeStreamOptions *options.ChangeStreamOptions) *options.ChangeStreamOptions {
	if resumeToken != nil {
		fmt.Println("Resuming change stream from previous token")
		changeStreamOptions = changeStreamOptions.SetResumeAfter(resumeToken)
	} else {
		// Fetch everything that is still retained in the oplog
		// Do not use this in production
		changeStreamOptions = changeStreamOptions.SetStartAtOperationTime(&primitive.Timestamp{T: 1})
	}
	return changeStreamOptions
}

func initializeMongoClient(ctx context.Context) error {
	if client == nil {
		var err error
		clientOptions := options.Client().
			ApplyURI("mongodb://localhost:27017/?replicaSet=rs0")
		client, err = mongo.Connect(ctx, clientOptions)
		if err != nil {
			return fmt.Errorf("failed to connect to MongoDB: %v", err)
		}
		fmt.Println("MongoDB client initialized")

		if err := client.Ping(ctx, nil); err != nil {
			return fmt.Errorf("failed to ping MongoDB: %v", err)
		}
		fmt.Println("MongoDB ping successful")
	}
	return nil
}

func disconnectMongoClient() {
	if client != nil {
		// Create a new context with timeout for disconnect operation, the application context might be cancelled
		disconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Disconnect(disconnectCtx); err != nil {
			fmt.Printf("Failed to disconnect MongoDB client: %v\n", err)
		} else {
			fmt.Println("MongoDB client disconnected")
		}
	}
}

func EnsureResumeTokenDirectoryExists() any {
	return os.MkdirAll(ResumeTokenDirectory, 0755)
}

func FetchResumeToken(ctx context.Context) (bson.Raw, error) {
	data, err := os.ReadFile(ResumeTokenFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return bson.Raw(data), nil
}

func StoreResumeToken(ctx context.Context, token bson.Raw) error {
	return os.WriteFile(ResumeTokenFilePath, token, 0644)
}
