#include <iostream>
#include <vector>
#include <string>
#include <algorithm>
#include <cstring>
#include <unistd.h>
#include <arpa/inet.h>

struct Packet {
    std::string Symbol;
    std::string BuySell;
    int32_t Quantity;
    int32_t Price;
    int32_t PacketSequence;
};

const std::string IP = "192.168.0.148";
const int PORT = 3000;
const int PACKET_SIZE = 17;

int createConnection(int &sock) {
    struct sockaddr_in server_address;

    sock = socket(AF_INET, SOCK_STREAM, 0);
    if (sock < 0) {
        std::cerr << "Failed to create socket." << std::endl;
        return -1;
    }

    server_address.sin_family = AF_INET;
    server_address.sin_port = htons(PORT);

    if (inet_pton(AF_INET, IP.c_str(), &server_address.sin_addr) <= 0) {
        std::cerr << "Invalid address." << std::endl;
        return -1;
    }

    if (connect(sock, (struct sockaddr *)&server_address, sizeof(server_address)) < 0) {
        std::cerr << "Connection failed." << std::endl;
        return -1;
    }

    return 0;
}

std::vector<Packet> receiveData(int sock) {
    std::vector<Packet> packets;
    char buffer[2048];

    while (true) {
        ssize_t bytes_read = read(sock, buffer, sizeof(buffer));
        if (bytes_read <= 0) {
            break;
        }

        for (ssize_t i = 0; i < bytes_read; i += PACKET_SIZE) {
            if (i + PACKET_SIZE <= bytes_read) {
                Packet packet;
                memcpy(&packet.Symbol, buffer + i, 4);
                packet.Symbol = packet.Symbol.substr(0, 4);

                packet.BuySell = std::string(1, buffer[i + 4]);
                memcpy(&packet.Quantity, buffer + i + 5, 4);
                memcpy(&packet.Price, buffer + i + 9, 4);
                memcpy(&packet.PacketSequence, buffer + i + 13, 4);

                packets.push_back(packet);
            } else {
                std::cerr << "Received an incomplete packet." << std::endl;
            }
        }
    }

    return packets;
}

std::vector<int32_t> findMissingSeq(const std::vector<Packet> &data) {
    std::vector<int32_t> sequences;
    for (const auto &packet : data) {
        sequences.push_back(packet.PacketSequence);
    }

    std::sort(sequences.begin(), sequences.end());

    std::vector<int32_t> missing;
    int32_t start_seq = 1;
    for (int32_t i = start_seq; i <= sequences.back(); ++i) {
        if (std::find(sequences.begin(), sequences.end(), i) == sequences.end()) {
            missing.push_back(i);
        }
    }

    return missing;
}

void resendPacket(int sock, const std::vector<int32_t> &resendSeqs) {
    char buffer[2048];

    for (const auto &seq : resendSeqs) {
        char header[2] = {2, static_cast<char>(seq)};

        if (write(sock, header, sizeof(header)) <= 0) {
            std::cerr << "Error sending stream request." << std::endl;
            continue;
        }

        ssize_t bytes_read = read(sock, buffer, sizeof(buffer));
        if (bytes_read <= 0) {
            std::cerr << "Error reading data." << std::endl;
            break;
        }

        for (ssize_t i = 0; i < bytes_read; i += PACKET_SIZE) {
            if (i + PACKET_SIZE <= bytes_read) {
                Packet packet;
                memcpy(&packet.Symbol, buffer + i, 4);
                packet.Symbol = packet.Symbol.substr(0, 4);

                packet.BuySell = std::string(1, buffer[i + 4]);
                memcpy(&packet.Quantity, buffer + i + 5, 4);
                memcpy(&packet.Price, buffer + i + 9, 4);
                memcpy(&packet.PacketSequence, buffer + i + 13, 4);

                std::cout << "Missed Packet Received: " << packet.PacketSequence << std::endl;
            } else {
                std::cerr << "Received an incomplete packet." << std::endl;
            }
        }
    }
}

int main() {
    int sock;
    if (createConnection(sock) != 0) {
        return -1;
    }

    char header[2] = {1, 0};
    if (write(sock, header, sizeof(header)) <= 0) {
        std::cerr << "Error sending stream request." << std::endl;
        return -1;
    }

    auto received_ticks = receiveData(sock);
    close(sock);

    std::this_thread::sleep_for(std::chrono::seconds(2));

    if (createConnection(sock) != 0) {
        return -1;
    }

    auto missed_seq = findMissingSeq(received_ticks);
    std::cout << "Missed sequences: ";
    for (const auto &seq : missed_seq) {
        std::cout << seq << " ";
    }
    std::cout << std::endl;

    resendPacket(sock, missed_seq);
    close(sock);

    return 0;
}
