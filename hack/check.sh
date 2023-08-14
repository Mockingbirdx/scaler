i=0
while true
do	
    echo -e "\n------<$i>------\n"
    date
    echo -e "\n"
    curl http://127.0.0.1:9000/
    echo -e "\n\n"
    i=`expr $i + 5`
    sleep 5m
done