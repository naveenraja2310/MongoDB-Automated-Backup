package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var AWSClient *s3.Client

func init() {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
}

func main() {
	InitializeS3Client()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "MongoDB Backup service is up...")
	})

	// Schedule the job to run at midnight (00:00)
	c := cron.New()
	c.AddFunc("0 0 * * *", func() {
		BackUp()
		UploadToS3()
		CleanExportsFolder()
	})
	c.Start()

	// Start the HTTP server on port 8080
	port := viper.GetString("APP_PORT")
	fmt.Println("Server listening on port ", fmt.Sprint(":", port))
	log.Fatal(http.ListenAndServe(fmt.Sprint(":", port), nil))

	fmt.Println("Backup uploaded to S3 successfully")
}

func BackUp() {
	// Load credentials from environment variables
	username := viper.GetString("MONGO_USERNAME")
	password := viper.GetString("MONGO_PASSWORD")
	clusterURI := viper.GetString("MONGO_CLUSTER_URI")
	outputDir := viper.GetString("BACKUP_OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "./backup"
	}

	// Build connection string
	connStr := fmt.Sprintf("mongodb+srv://%s:%s@%s", username, password, clusterURI)

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clientOpts := options.Client().ApplyURI(connStr)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		fmt.Printf("Failed to connect to MongoDB: %v\n", err)
		return
	}
	defer client.Disconnect(ctx)

	// Get list of database names
	dbs, err := client.ListDatabaseNames(ctx, map[string]interface{}{})
	if err != nil {
		fmt.Printf("Failed to list databases: %v\n", err)
		return
	}

	// Loop through databases and run mongodump
	for _, dbName := range dbs {
		// Skip internal databases (optional)
		if dbName == "admin" || dbName == "local" || dbName == "config" {
			continue
		}

		fmt.Printf("Backing up database: %s\n", dbName)
		cmd := exec.Command("mongodump",
			"--uri", fmt.Sprintf("mongodb+srv://%s:%s@%s/%s", username, password, clusterURI, dbName),
			"--out", fmt.Sprintf("%s/%s", outputDir, dbName),
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("Failed to dump %s: %v\n", dbName, err)
		} else {
			fmt.Printf("Successfully backed up %s\n", dbName)
		}
	}

	fmt.Println("All backups completed.")
}

func CleanExportsFolder() error {
	dir := viper.GetString("BACKUP_OUTPUT_DIR")
	if dir == "" {
		dir = "./backup"
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		err := os.RemoveAll(path) // Removes both files and directories
		if err != nil {
			return err
		}
	}

	return nil
}

func InitializeS3Client() {
	awsCfg, err := CreateAWSConfig()
	if err != nil {
		fmt.Printf("unable to load AWS config: %v", err)
	}

	AWSClient = s3.NewFromConfig(awsCfg)
}

func CreateAWSConfig() (aws.Config, error) {
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(viper.GetString("AWS_REGION")),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			viper.GetString("AWS_ACCESS_KEY_ID"),
			viper.GetString("AWS_SECRET_ACCESS_KEY"),
			"",
		)),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("unable to load AWS config: %w", err)
	}

	return awsCfg, nil
}

func UploadToS3() error {
	// Zip the backup folder
	dir := viper.GetString("BACKUP_OUTPUT_DIR")
	if dir == "" {
		dir = "./backup"
	}
	zipPath := "mongodb-dump-" + time.Now().Format("2006-01-02") + ".zip"
	if err := ZipFolder(dir, zipPath); err != nil {
		return fmt.Errorf("failed to zip backup folder: %w", err)
	}

	// Open the zip file
	file, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zipped backup: %w", err)
	}

	// Read content type
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read from zip file: %w", err)
	}
	contentType := http.DetectContentType(buffer)

	// Reset pointer
	_, err = file.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to beginning of zip file: %w", err)
	}

	imagekey := zipPath

	// Upload to S3
	_, err = AWSClient.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(viper.GetString("AWS_BUCKET_NAME")),
		Key:         aws.String(imagekey),
		Body:        file,
		ContentType: aws.String(contentType),
	})

	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	fmt.Println("Backup uploaded to S3 successfully as", imagekey)
	file.Close()
	// Attempt to remove the file
	removeerr := os.Remove(zipPath)
	if removeerr != nil {
		// Handle the error, e.g., if the file doesn't exist or permissions are insufficient
		if os.IsNotExist(removeerr) {
			fmt.Printf("File not found: %s\n", zipPath)
		} else {
			log.Fatalf("Error removing file %s: %v\n", zipPath, removeerr)
		}
	} else {
		fmt.Printf("File %s removed successfully.\n", zipPath)
	}

	return nil
}

func ZipFolder(source, target string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(writer, file)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return err
}
