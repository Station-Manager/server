# Station Manager: server

## 

1. User registers: email, password
2. User creates a Logbook:
    - name
    - description
    - callsign
    - API key - can only be used for this logbook and the callsign MUST match the station_callsign, etc.

3. User uploads QSOs (realtime)
    - API key
    - ADIF data (minimum required fields):
        - callsign
        - frequency
        - mode
        - band
        - qso_date
        - time_on
        - time_off
        - rst_sent
        - rst_rcvd

4. 