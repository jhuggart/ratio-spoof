import bencode_parser
import sys 
import hashlib
import urllib.parse
import json
import random 
import base64
import os
import uuid
import argparse
import time
from collections import deque
import subprocess
import platform
import datetime



def t_total_size(data):
    return sum(map(lambda x : x['length'] , data['info']['files']))

def t_infohash_urlencoded(data, raw_data):
    info_offsets= data['info']['byte_offsets']
    info_bytes = hashlib.sha1(raw_data[info_offsets[0]:info_offsets[1]]).digest()
    return urllib.parse.quote(info_bytes).lower()

def t_piecesize_b(data):
    return data['info']['piece length']

def next_announce_total_b(kb_speed, b_current, b_piece_size,s_time, b_total_limit = None):
    total = b_current + (kb_speed *1024 *s_time)
    closest_piece_number = int(total / b_piece_size)
    closest_piece_number = closest_piece_number + random.randint(-10,10)
    next_announce = closest_piece_number *b_piece_size
    if(b_total_limit is not None and next_announce > b_total_limit):
        return b_total_limit    
    return next_announce

def next_announce_left_b(b_current, b_total_size):
    return b_total_size - b_current


def peer_id():
    peer_id =  f'-qB4030-{base64.urlsafe_b64encode(uuid.uuid4().bytes)[:12].decode()}'
    return peer_id

def find_approx_current(b_total_size, piece_size, percent):
    if( percent <= 0): return 0
    total = (percent/100) * b_total_size
    current_approx = int(total / piece_size) * piece_size
    return current_approx

def print_header(torrent_name, percent_complete, download_speed, upload_speed):
    sys.stdout.write(f"""
###########################################################################
    Torrent: {torrent_name} - {percent_complete}% 
    download_speed: {download_speed}KB/s 
    upload_speed: {upload_speed}KB/s
###########################################################################
"""
    )

def print_body(d: deque, time):
    for item in list(d)[:len(d)-1]:
        print(f'#{item["count"]} downloaded: {item["downloaded"]}  | uploaded: {item["uploaded"]} | announced')
    print(f'#{d[-1]["count"]} downloaded: {d[-1]["downloaded"]}  | uploaded: {d[-1]["uploaded"]} | next announce in :{str(datetime.timedelta(seconds=time))}')

def clear_screen():
    if platform.system() == "Windows":
        subprocess.Popen("cls", shell=True).communicate()
    else:
        print("\033c", end="")

def read_file(f, args_downalod, args_upload):
    raw_data = f.read()
    result = bencode_parser.decode(raw_data)
    piece_size = t_piecesize_b(result)
    total_size = t_total_size(result) 
    current_downloaded = find_approx_current(total_size,piece_size,args_downalod[0])
    current_uploaded = find_approx_current(total_size,piece_size,args_upload[0])
    delay_announce =18000
    deq = deque(maxlen=10)
    count = 1
    while True:
        if(current_downloaded < total_size):
            current_downloaded = next_announce_total_b(args_downalod[1],current_downloaded, piece_size, delay_announce, total_size)
        current_uploaded = next_announce_total_b(args_upload[1],current_uploaded, piece_size, delay_announce)
        deq.append({'count': count, 'downloaded':current_downloaded,'uploaded':current_uploaded })
        count +=1
        clear_screen()
        print_header(result['info']['name'], args_downalod[0], args_downalod[1], args_upload[1])
        print_body(deq, delay_announce)
        delay_announce = delay_announce - 1 if delay_announce > 0 else 0
        time.sleep(1)

        


parser = argparse.ArgumentParser()
parser.add_argument('-t', required=True,help='path .torrent file' , type=argparse.FileType('rb'))
parser.add_argument('-d', required=True,type=int,help='parms for download', nargs=2 ,metavar=('%_COMPLETE', 'KBS_SPEED'))
parser.add_argument('-u',required=True,type=int,help='parms for upload', nargs=2 ,metavar=('%_COMPLETE', 'KBS_SPEED'))
args = parser.parse_args()
read_file(args.t, args.d, args.u)
