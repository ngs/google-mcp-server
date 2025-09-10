# Google OAuth Setup Guide for Google MCP Server

This guide will help you resolve the 403 access_denied error when authenticating with Google OAuth.

## Step 1: Enable Required APIs

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Select your project (or create a new one)
3. Navigate to **APIs & Services > Library**
4. Search for and enable each of these APIs:
   - **Google Calendar API**
   - **Google Drive API**
   - **Gmail API**
   - **Google Sheets API**
   - **Google Docs API**

## Step 2: Configure OAuth Consent Screen

1. Go to **APIs & Services > OAuth consent screen**
2. Select User Type:
   - Choose **External** for personal Google accounts
   - Choose **Internal** for Google Workspace accounts (if available)
3. Click **CREATE**

### App Information
Fill in the required fields:
- **App name**: Google MCP Server (or your preferred name)
- **User support email**: Your email address
- **App logo**: Optional

### App Domain
- You can skip these fields for local development

### Developer Contact Information
- **Email addresses**: Your email address

Click **SAVE AND CONTINUE**

### Scopes
1. Click **ADD OR REMOVE SCOPES**
2. Add these scopes (search or paste them):
   - `https://www.googleapis.com/auth/calendar`
   - `https://www.googleapis.com/auth/drive`
   - `https://www.googleapis.com/auth/gmail.modify`
   - `https://www.googleapis.com/auth/spreadsheets`
   - `https://www.googleapis.com/auth/documents`
   - `https://www.googleapis.com/auth/userinfo.email`
   - `https://www.googleapis.com/auth/userinfo.profile`
   - `openid`

3. Click **UPDATE** and then **SAVE AND CONTINUE**

### Test Users (Important!)
If your app is in "Testing" mode:
1. Click **ADD USERS**
2. Add your Google account email address
3. Add any other email addresses that will use the app
4. Click **ADD**
5. Click **SAVE AND CONTINUE**

## Step 3: Create OAuth 2.0 Client ID

1. Go to **APIs & Services > Credentials**
2. Click **+ CREATE CREDENTIALS** > **OAuth client ID**
3. Select **Application type**: **Desktop app**
4. **Name**: Google MCP Server Client (or your preferred name)
5. Click **CREATE**
6. Download the JSON file or copy the Client ID and Client Secret

## Step 4: Configure the MCP Server

### Option A: Using config.json

Create or edit `~/.google-mcp-server/config.json`:

```json
{
  "oauth": {
    "client_id": "YOUR_CLIENT_ID.apps.googleusercontent.com",
    "client_secret": "YOUR_CLIENT_SECRET",
    "redirect_uri": "http://localhost:8080/callback"
  },
  "services": {
    "calendar": {"enabled": true},
    "drive": {"enabled": true},
    "gmail": {"enabled": true},
    "sheets": {"enabled": true},
    "docs": {"enabled": true}
  }
}
```

### Option B: Using Environment Variables

```bash
export GOOGLE_CLIENT_ID="YOUR_CLIENT_ID.apps.googleusercontent.com"
export GOOGLE_CLIENT_SECRET="YOUR_CLIENT_SECRET"
```

## Step 5: Authenticate

1. Run the server to initiate OAuth flow:
   ```bash
   google-mcp-server
   ```

2. A browser window will open automatically
3. Sign in with your Google account (must be in the test users list if app is in testing mode)
4. Review the permissions and click **Allow**
5. The token will be saved to `~/.google-mcp-accounts/<your-email>.json`

### Multi-Account Support

The server supports multiple Google accounts:
- First authentication creates the default account
- Use the `accounts_add` tool to add additional accounts
- Each account's token is stored separately in `~/.google-mcp-accounts/`
- The server automatically selects the appropriate account based on context
- Use `accounts_list` to see all authenticated accounts

## Common Issues and Solutions

### Still Getting 403 access_denied?

1. **Verify Test Users**: If your OAuth app is in "Testing" mode, make sure your email is added to the test users list

2. **Check Publishing Status**: 
   - Go to **OAuth consent screen**
   - Check if the app status is "Testing" or "Published"
   - For testing, only test users can authenticate
   - To publish (for personal use), click **PUBLISH APP**

3. **Wait for Propagation**: After enabling APIs or changing OAuth settings, wait 5-10 minutes for changes to propagate

4. **Verify Scopes**: Ensure all required scopes are added in the OAuth consent screen configuration

5. **Check API Quotas**: Some APIs have daily quotas that might be exceeded

### Google Workspace Restrictions

If you're using a Google Workspace account:
- Your administrator may need to approve the app
- Some organizations restrict third-party app access
- Contact your Google Workspace administrator if you continue to have issues

### Regenerate Credentials

If all else fails:
1. Delete the existing OAuth client
2. Create a new OAuth client
3. Update your config with new credentials
4. Delete the old tokens: `rm -rf ~/.google-mcp-accounts/`
5. Re-authenticate

## Security Notes

- Never commit your OAuth credentials to version control
- Keep your `client_secret` secure
- Token files in `~/.google-mcp-accounts/` contain sensitive data - protect them with appropriate file permissions (automatically set to 0600)

## Need Help?

If you continue to experience issues:
1. Check the [Google Cloud Console logs](https://console.cloud.google.com/logs)
2. Verify all APIs are enabled in the [API Library](https://console.cloud.google.com/apis/library)
3. Review the [OAuth consent screen configuration](https://console.cloud.google.com/apis/credentials/consent)
4. File an issue at [GitHub](https://github.com/ngs/google-mcp-server/issues)