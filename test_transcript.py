#!/usr/bin/env python3

import sys
import json
from youtube_transcript_api import YouTubeTranscriptApi
from youtube_transcript_api._errors import TranscriptsDisabled, NoTranscriptFound

def test_video_transcript(video_id):
    print(f"Testing video: {video_id}")
    
    try:
        # Initialize the API
        ytt_api = YouTubeTranscriptApi()
        
        # First, try to list all available transcripts
        print("Checking available transcripts...")
        transcript_list = ytt_api.list(video_id)
        
        print("Available transcripts:")
        for transcript in transcript_list:
            print(f"  - {transcript.language_code} ({transcript.language}) - Generated: {transcript.is_generated}")
        
        # Try to fetch English transcript
        print("\nTrying to fetch English transcript...")
        transcript = ytt_api.fetch(video_id, languages=['en','fr'])
        
        print(f"Success! Found transcript with {len(transcript)} snippets")
        print("First few snippets:")
        for i, snippet in enumerate(transcript[:3]):
            print(f"  {snippet.start:.2f}s: {snippet.text}")
        
        return True
        
    except TranscriptsDisabled:
        print("❌ Transcripts are disabled for this video")
        return False
    except NoTranscriptFound:
        print("❌ No transcript found in requested languages")
        return False
    except Exception as e:
        print(f"❌ Error: {e}")
        return False

if __name__ == "__main__":
    if len(sys.argv) > 1:
        video_id = sys.argv[1]
    else:
        video_id = "NnYLzGMk8Tg"
    test_video_transcript(video_id)
