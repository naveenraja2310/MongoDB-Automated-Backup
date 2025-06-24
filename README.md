# MongoDB Automated Backup Service to S3

This Go-based service automatically backs up all non-system MongoDB databases from a MongoDB Atlas cluster, compresses the backups, and uploads them to an AWS S3 bucket. The process is scheduled to run **daily at midnight (00:00)** using a cron job and also exposes a simple health check HTTP server.

## 🚀 Features

- Connects to MongoDB Atlas using credentials from `.env`
- Loops through all databases and performs `mongodump` on each
- Skips internal MongoDB databases (`admin`, `local`, `config`)
- Compresses backup folder into a zip file
- Uploads the zipped file to S3
- Automatically deletes the backup and zipped file after upload
- Cron job runs every day at midnight
- HTTP server listens on `/` to indicate the service is running

## 🛠 How It Works

1. At **00:00 (midnight)**, the `cron` scheduler triggers the backup process.
2. The app:
   - Connects to the MongoDB Atlas cluster
   - Fetches all database names (except internal ones)
   - Runs `mongodump` per database into the `./backup` folder
   - Compresses the backup into a `.zip` file
   - Uploads it to the configured AWS S3 bucket
   - Deletes the local backup folder and `.zip` file
3. An HTTP server runs on `localhost:8080` (configurable) for monitoring.

## 📦 Environment Variables

Set these variables in a `.env` file in the root directory:

```env
# MongoDB Credentials
MONGO_USERNAME=your_mongo_username
MONGO_PASSWORD=your_mongo_password
MONGO_CLUSTER_URI=your_cluster.mongodb.net
BACKUP_OUTPUT_DIR=./backup

# AWS Credentials
AWS_ACCESS_KEY_ID=your_aws_access_key_id
AWS_SECRET_ACCESS_KEY=your_aws_secret_access_key
AWS_REGION=ap-south-1
AWS_BUCKET_NAME=your-s3-bucket-name

# App Port
APP_PORT=8080
```

## 💻 Getting Started

### 1. Install Dependencies

```bash
go mod tidy
```

### 2. Install MongoDB Tools (`mongodump`)

**For Ubuntu:**
```bash
wget https://fastdl.mongodb.org/tools/db/mongodb-database-tools-ubuntu2204-x86_64-100.9.4.deb
sudo dpkg -i mongodb-database-tools-*.deb
```

**For macOS:**
```bash
brew install mongodb/brew/mongodb-database-tools
```

**For Windows:**
Download from [MongoDB Database Tools](https://www.mongodb.com/try/download/database-tools) and add to PATH.

**Verify installation:**
```bash
mongodump --version
```

### 3. Configure Environment

Create a `.env` file in the project root and add your credentials as shown in the Environment Variables section.

### 4. Run the App

```bash
go run main.go
```

### 5. Confirm it's running

Visit: [http://localhost:8080](http://localhost:8080)

You should see: `MongoDB Backup service is up...`

## 🗂 File Structure

```
.
├── main.go
├── .env
├── go.mod
├── go.sum
├── README.md
├── backup/               # Temporary folder to hold dump (created automatically)
└── mongodb-dump-*.zip    # Created zip file (deleted after upload)
```

## 🔁 Cron Behavior

- Uses [`robfig/cron`](https://pkg.go.dev/github.com/robfig/cron) to schedule backups
- Schedule: `0 0 * * *` (every day at midnight)
- Backup is initiated without manual intervention
- Timezone: Uses system timezone

## ☁️ AWS S3 Notes

- The `.zip` file will be uploaded to the S3 bucket specified in `AWS_BUCKET_NAME`
- File name pattern: `mongodb-dump-YYYY-MM-DD-HHMMSS.zip`
- Files are automatically removed from the local server after successful upload
- Ensure your S3 bucket has appropriate permissions for the IAM user

## ✅ Health Check

The app runs a lightweight HTTP server to confirm it's alive:

```bash
curl http://localhost:8080
# Output: MongoDB Backup service is up...
```

## 🔧 Dependencies

Add these to your `go.mod`:

```go
require (
    github.com/aws/aws-sdk-go v1.44.0
    github.com/joho/godotenv v1.4.0
    github.com/robfig/cron/v3 v3.0.1
    go.mongodb.org/mongo-driver v1.11.0
)
```

## 📌 Requirements

- Go 1.18+
- MongoDB Atlas credentials with read access to databases
- AWS S3 bucket and IAM access keys with S3 write permissions
- `mongodump` tool installed on your system
- Network connectivity to MongoDB Atlas and AWS S3

## 🚨 Important Notes

- Ensure your MongoDB user has the necessary permissions to read all databases
- The service creates temporary files during backup process - ensure sufficient disk space
- Monitor S3 costs as backup files can accumulate over time
- Consider implementing backup retention policies in S3
- Test the backup and restore process regularly

## 🐛 Troubleshooting

### Common Issues

1. **`mongodump` command not found**
   - Ensure MongoDB Database Tools are installed and in PATH

2. **MongoDB connection failed**
   - Verify credentials in `.env` file
   - Check network connectivity to MongoDB Atlas
   - Ensure IP whitelist includes your server

3. **S3 upload failed**
   - Verify AWS credentials and permissions
   - Check S3 bucket name and region
   - Ensure bucket exists and is accessible

4. **Backup folder permissions**
   - Ensure the application has write permissions in the backup directory

## 📄 License

This project is licensed under the MIT License.
